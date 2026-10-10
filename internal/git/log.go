package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// FileChange is one numstat entry of a commit.
type FileChange struct {
	Path    string // new path for renames
	OldPath string // set only for renames
	Added   int
	Deleted int
	Binary  bool // counts are zero; the commit still touched the file
}

// Commit is a parsed git log record. Names and emails are the mailmapped forms.
type Commit struct {
	Hash        string
	AuthorName  string
	AuthorEmail string
	// Time carries the author's own UTC offset so hour-of-day stats can use
	// the author's local time.
	Time          time.Time
	CommitterName string
	Subject       string
	Files         []FileChange
}

// LogOptions controls which history is read.
type LogOptions struct {
	Rev           string // default HEAD
	Since, Until  string // passed through to git log
	IncludeMerges bool
}

// Log reads history via git log --numstat and returns commits newest first.
// A repository without commits yields an empty slice, not an error.
func Log(ctx context.Context, repo string, opts LogOptions) ([]Commit, error) {
	rev := opts.Rev
	if rev == "" {
		rev = "HEAD"
	}
	if !HasCommits(ctx, repo, rev) {
		return nil, nil
	}
	args := []string{"log", "--use-mailmap", "--numstat", "-z",
		"--date=format:%z",
		"--format=%x1e%H%x1f%aN%x1f%aE%x1f%at%x1f%ad%x1f%cN%x1f%s"}
	if !opts.IncludeMerges {
		args = append(args, "--no-merges")
	}
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}
	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}
	args = append(args, rev, "--")

	cmd := gitCmd(ctx, repo, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("log %s: %w", repo, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("log %s: %w", repo, err)
	}
	commits, parseErr := ParseLog(stdout)
	// Drain so Wait cannot block on a full pipe if parsing bailed early.
	_, _ = io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if waitErr != nil {
		return nil, fmt.Errorf("log %s: %w: %s", repo, waitErr, strings.TrimSpace(stderr.String()))
	}
	if parseErr != nil {
		return nil, fmt.Errorf("log %s: %w", repo, parseErr)
	}
	return commits, nil
}

// ParseLog parses the -z output of Log one record at a time, so memory is
// bounded by the largest single commit rather than the whole history.
func ParseLog(r io.Reader) ([]Commit, error) {
	br := bufio.NewReaderSize(r, 1<<20)
	var commits []Commit
	for {
		rec, err := br.ReadBytes(0x1e)
		atEOF := err == io.EOF
		if err != nil && !atEOF {
			return nil, err
		}
		rec = bytes.TrimSuffix(rec, []byte{0x1e})
		if len(rec) > 0 {
			c, perr := parseRecord(rec)
			if perr != nil {
				return nil, perr
			}
			commits = append(commits, c)
		}
		if atEOF {
			return commits, nil
		}
	}
}

func parseRecord(rec []byte) (Commit, error) {
	header, rest, _ := bytes.Cut(rec, []byte{0})
	f := strings.SplitN(string(header), "\x1f", 7)
	if len(f) != 7 {
		return Commit{}, fmt.Errorf("malformed commit header %q", header)
	}
	unix, err := strconv.ParseInt(f[3], 10, 64)
	if err != nil {
		return Commit{}, fmt.Errorf("commit %s: bad timestamp %q", f[0], f[3])
	}
	c := Commit{
		Hash:          f[0],
		AuthorName:    f[1],
		AuthorEmail:   f[2],
		Time:          time.Unix(unix, 0).In(zoneFromOffset(f[4])),
		CommitterName: f[5],
		Subject:       f[6],
	}
	c.Files, err = parseNumstat(bytes.TrimLeft(rest, "\n"))
	if err != nil {
		return Commit{}, fmt.Errorf("commit %s: %w", c.Hash, err)
	}
	return c, nil
}

// zoneFromOffset turns "+0530" into a fixed zone; unparsable input is UTC.
func zoneFromOffset(s string) *time.Location {
	if len(s) != 5 || (s[0] != '+' && s[0] != '-') {
		return time.UTC
	}
	h, err1 := strconv.Atoi(s[1:3])
	m, err2 := strconv.Atoi(s[3:5])
	if err1 != nil || err2 != nil {
		return time.UTC
	}
	secs := h*3600 + m*60
	if s[0] == '-' {
		secs = -secs
	}
	return time.FixedZone(s, secs)
}

// parseNumstat handles the NUL-separated numstat body. Normal entries are
// "A\tD\tpath"; renames are "A\tD\t" followed by the old and new path as two
// further NUL-separated tokens.
func parseNumstat(body []byte) ([]FileChange, error) {
	if len(body) == 0 {
		return nil, nil
	}
	toks := strings.Split(strings.TrimSuffix(string(body), "\x00"), "\x00")
	var files []FileChange
	for i := 0; i < len(toks); i++ {
		tok := toks[i]
		if tok == "" {
			continue
		}
		parts := strings.SplitN(tok, "\t", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed numstat entry %q", tok)
		}
		fc := FileChange{Path: parts[2]}
		if parts[0] == "-" && parts[1] == "-" {
			fc.Binary = true
		} else {
			var e1, e2 error
			fc.Added, e1 = strconv.Atoi(parts[0])
			fc.Deleted, e2 = strconv.Atoi(parts[1])
			if e1 != nil || e2 != nil {
				return nil, fmt.Errorf("bad numstat counts in %q", tok)
			}
		}
		if parts[2] == "" {
			if i+2 >= len(toks) {
				return nil, fmt.Errorf("truncated rename entry %q", tok)
			}
			fc.OldPath, fc.Path = toks[i+1], toks[i+2]
			i += 2
		} else if strings.Contains(fc.Path, " => ") {
			// Only seen without -z; resolve defensively.
			fc.OldPath, fc.Path = ResolveRename(fc.Path)
		}
		files = append(files, fc)
	}
	return files, nil
}

// ResolveRename expands git's compact rename notation ("old => new" or
// "dir/{a => b}/c") into its old and new paths.
func ResolveRename(p string) (oldPath, newPath string) {
	open := strings.Index(p, "{")
	closeIdx := strings.Index(p, "}")
	if open >= 0 && closeIdx > open {
		inner := p[open+1 : closeIdx]
		from, to, ok := strings.Cut(inner, " => ")
		if ok {
			prefix, suffix := p[:open], p[closeIdx+1:]
			return cleanSlashes(prefix + from + suffix), cleanSlashes(prefix + to + suffix)
		}
	}
	if from, to, ok := strings.Cut(p, " => "); ok {
		return from, to
	}
	return p, p
}

// cleanSlashes collapses the "//" left behind when a braced side is empty,
// as in "dir/{ => sub}/f".
func cleanSlashes(p string) string {
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return p
}
