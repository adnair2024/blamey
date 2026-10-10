package git

import (
	"bufio"
	"context"
	"path"
	"regexp"
	"strings"
	"sync"
)

var defaultExcludedDirs = []string{"vendor", "node_modules", "third_party", "dist", "build"}

var defaultExcludedFiles = map[string]bool{
	"go.sum": true, "package-lock.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "Cargo.lock": true, "poetry.lock": true,
}

var defaultExcludedSuffixes = []string{".pb.go", "_gen.go", ".min.js"}

// Filter decides which paths count towards the statistics. Generated and
// vendored code inflates numbers, so the defaults exclude it and the user opts
// in with Include. Safe for concurrent use.
type Filter struct {
	include []*regexp.Regexp
	exclude []*regexp.Regexp

	mu      sync.Mutex
	flagged map[string]bool // excluded by content or .gitattributes
}

// NewFilter compiles --include / --exclude globs. "**" crosses directories;
// a pattern without "/" matches any path component or basename.
func NewFilter(include, exclude []string) *Filter {
	f := &Filter{flagged: map[string]bool{}}
	for _, g := range include {
		f.include = append(f.include, globToRegexp(g))
	}
	for _, g := range exclude {
		f.exclude = append(f.exclude, globToRegexp(g))
	}
	return f
}

// Allow reports whether the path is counted. Explicit excludes always win;
// includes rescue a path from the default exclusions only.
func (f *Filter) Allow(p string) bool {
	if matchAny(f.exclude, p) {
		return false
	}
	if matchAny(f.include, p) {
		return true
	}
	if defaultExcluded(p) {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.flagged[p]
}

func (f *Filter) explicitlyIncluded(p string) bool {
	return matchAny(f.include, p) && !matchAny(f.exclude, p)
}

// MarkExcluded records a path found to be generated or vendored by inspecting
// its content or attributes, which are only known after reading the file.
func (f *Filter) MarkExcluded(p string) {
	f.mu.Lock()
	f.flagged[p] = true
	f.mu.Unlock()
}

func defaultExcluded(p string) bool {
	if defaultExcludedFiles[path.Base(p)] {
		return true
	}
	for _, s := range defaultExcludedSuffixes {
		if strings.HasSuffix(p, s) {
			return true
		}
	}
	dir := path.Dir(p)
	for _, seg := range strings.Split(dir, "/") {
		for _, d := range defaultExcludedDirs {
			if seg == d {
				return true
			}
		}
	}
	return false
}

// HasGeneratedHeader reports whether the first five lines mark the file as
// generated ("Code generated ... DO NOT EDIT").
func HasGeneratedHeader(lines []string) bool {
	for i, l := range lines {
		if i >= 5 {
			break
		}
		if strings.Contains(l, "Code generated") && strings.Contains(l, "DO NOT EDIT") {
			return true
		}
	}
	return false
}

func matchAny(res []*regexp.Regexp, p string) bool {
	for _, re := range res {
		if re.MatchString(p) {
			return true
		}
	}
	return false
}

// globToRegexp translates a glob to an anchored regexp over slash paths.
func globToRegexp(glob string) *regexp.Regexp {
	glob = strings.TrimPrefix(glob, "./")
	anchored := strings.Contains(strings.TrimSuffix(glob, "/"), "/")
	dirOnly := strings.HasSuffix(glob, "/")
	glob = strings.TrimSuffix(glob, "/")

	var b strings.Builder
	if anchored {
		b.WriteString("^")
	} else {
		b.WriteString("(?:^|/)")
	}
	for i := 0; i < len(glob); i++ {
		switch c := glob[i]; c {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				i++
				if i+1 < len(glob) && glob[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	if dirOnly {
		b.WriteString("/")
	} else {
		b.WriteString("(?:$|/)")
	}
	re, err := regexp.Compile(b.String())
	if err != nil {
		return regexp.MustCompile(`$^`) // matches nothing
	}
	return re
}

// attrExcluded asks git which paths carry linguist-generated or
// linguist-vendored in .gitattributes, honoring nested attribute files.
func attrExcluded(ctx context.Context, repo string, paths []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(paths) == 0 {
		return out, nil
	}
	in := strings.Join(paths, "\x00") + "\x00"
	raw, err := runGit(ctx, repo, []byte(in), "check-attr", "-z", "--stdin",
		"linguist-generated", "linguist-vendored")
	if err != nil {
		return nil, err
	}
	toks := strings.Split(string(raw), "\x00")
	for i := 0; i+2 < len(toks); i += 3 {
		switch toks[i+2] {
		case "set", "true":
			out[toks[i]] = true
		}
	}
	return out, nil
}

// binaryFiles lists binary files at rev by diffing against the empty tree,
// where numstat reports "-\t-" for binaries.
func binaryFiles(ctx context.Context, repo, rev string) (map[string]bool, error) {
	empty, err := runGit(ctx, repo, []byte{}, "hash-object", "-t", "tree", "--stdin")
	if err != nil {
		return nil, err
	}
	raw, err := runGit(ctx, repo, nil, "diff", "--numstat", "-z", "--no-renames",
		strings.TrimSpace(string(empty)), rev, "--")
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	sc.Split(scanNUL)
	for sc.Scan() {
		if parts := strings.SplitN(sc.Text(), "\t", 3); len(parts) == 3 && parts[0] == "-" && parts[1] == "-" {
			out[parts[2]] = true
		}
	}
	return out, sc.Err()
}

func scanNUL(data []byte, atEOF bool) (int, []byte, error) {
	for i, b := range data {
		if b == 0 {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
