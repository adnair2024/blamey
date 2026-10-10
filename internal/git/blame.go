package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Author is a raw (mailmapped) author as seen by blame.
type Author struct {
	Name, Email string
}

// FileBlame is the per-file ownership result: surviving lines per author.
type FileBlame struct {
	Path      string
	Lines     map[Author]int
	Total     int
	LastTime  int64 // newest author-time among surviving lines, unix seconds
	Generated bool  // header says "Code generated ... DO NOT EDIT"
}

// BlameCache stores results keyed by (path, blob sha) so unchanged files are
// never re-blamed. Implementations must be safe for concurrent use and must
// treat any failure as a miss.
type BlameCache interface {
	Get(path, blob string) (FileBlame, bool)
	Put(path, blob string, fb FileBlame)
}

// TreeFile is a blob at the blamed revision.
type TreeFile struct {
	Path string
	Blob string
}

// ListFiles returns the regular files at rev that pass the filter and are
// neither binary nor marked linguist-generated / linguist-vendored. Content
// based "Code generated" detection happens later, during blame.
func ListFiles(ctx context.Context, repo, rev string, f *Filter) ([]TreeFile, error) {
	raw, err := runGit(ctx, repo, nil, "ls-tree", "-r", "-z", rev)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	var cand []TreeFile
	for _, rec := range strings.Split(string(raw), "\x00") {
		meta, p, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta) // mode type sha
		// Skip submodules (160000) and symlinks (120000): not blameable text.
		if len(fields) != 3 || fields[1] != "blob" || fields[0] == "120000" {
			continue
		}
		if f.Allow(p) {
			cand = append(cand, TreeFile{Path: p, Blob: fields[2]})
		}
	}
	paths := make([]string, len(cand))
	for i, c := range cand {
		paths[i] = c.Path
	}
	bins, err := binaryFiles(ctx, repo, rev)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	attrs, err := attrExcluded(ctx, repo, paths)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	out := cand[:0]
	for _, c := range cand {
		if bins[c.Path] {
			continue
		}
		if attrs[c.Path] {
			f.MarkExcluded(c.Path)
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// BlameOptions configures BlameAll.
type BlameOptions struct {
	Rev      string
	Jobs     int // default runtime.NumCPU()
	Cache    BlameCache
	Progress func(done, total int) // called from worker goroutines; must be cheap
	Filter   *Filter               // optional; receives MarkExcluded for generated files
}

// BlameAll blames every file in a bounded worker pool. Files whose header marks
// them generated are dropped from the result (and flagged on the Filter) unless
// the filter explicitly includes them.
func BlameAll(ctx context.Context, repo string, files []TreeFile, opts BlameOptions) (map[string]FileBlame, error) {
	rev := opts.Rev
	if rev == "" {
		rev = "HEAD"
	}
	jobs := opts.Jobs
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	ignoreFile := ""
	if _, err := os.Stat(filepath.Join(repo, ".git-blame-ignore-revs")); err == nil {
		ignoreFile = ".git-blame-ignore-revs"
	}

	var (
		mu   sync.Mutex
		done int
		res  = make(map[string]FileBlame, len(files))
	)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(jobs)
	for _, tf := range files {
		g.Go(func() error {
			fb, err := blameOne(gctx, repo, rev, ignoreFile, tf, opts.Cache)
			if err != nil {
				return err
			}
			mu.Lock()
			done++
			n := done
			if fb.Generated && opts.Filter != nil && opts.Filter.explicitlyIncluded(tf.Path) {
				fb.Generated = false
			}
			if !fb.Generated {
				res[tf.Path] = fb
			} else if opts.Filter != nil {
				opts.Filter.MarkExcluded(tf.Path)
			}
			mu.Unlock()
			if opts.Progress != nil {
				opts.Progress(n, len(files))
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return res, nil
}

func blameOne(ctx context.Context, repo, rev, ignoreFile string, tf TreeFile, c BlameCache) (FileBlame, error) {
	if c != nil {
		if fb, ok := c.Get(tf.Path, tf.Blob); ok {
			return fb, nil
		}
	}
	args := []string{"blame", "--line-porcelain", "-w", "-M", "-C"}
	if ignoreFile != "" {
		args = append(args, "--ignore-revs-file", ignoreFile)
	}
	args = append(args, rev, "--", tf.Path)

	cmd := gitCmd(ctx, repo, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return FileBlame{}, fmt.Errorf("blame %s: %w", tf.Path, err)
	}
	if err := cmd.Start(); err != nil {
		return FileBlame{}, fmt.Errorf("blame %s: %w", tf.Path, err)
	}
	fb, parseErr := ParseBlame(stdout)
	_, _ = io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return FileBlame{}, ctx.Err()
	}
	if waitErr != nil {
		return FileBlame{}, fmt.Errorf("blame %s: %w: %s", tf.Path, waitErr, strings.TrimSpace(stderr.String()))
	}
	if parseErr != nil {
		return FileBlame{}, fmt.Errorf("blame %s: %w", tf.Path, parseErr)
	}
	fb.Path = tf.Path
	if c != nil {
		c.Put(tf.Path, tf.Blob, fb)
	}
	return fb, nil
}

// ParseBlame reads --line-porcelain output. With --line-porcelain every line
// repeats its full header, so each group is self-contained and the group ends
// at the tab-prefixed content line.
func ParseBlame(r io.Reader) (FileBlame, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	fb := FileBlame{Lines: map[Author]int{}}
	var cur Author
	var curTime int64
	var head []string // first five content lines, for the generated check
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSuffix(line, "\n")
			switch {
			case strings.HasPrefix(line, "\t"):
				fb.Lines[cur]++
				fb.Total++
				if curTime > fb.LastTime {
					fb.LastTime = curTime
				}
				if len(head) < 5 {
					head = append(head, line[1:])
				}
			case strings.HasPrefix(line, "author "):
				cur = Author{Name: line[len("author "):]}
			case strings.HasPrefix(line, "author-mail "):
				cur.Email = strings.Trim(line[len("author-mail "):], "<>")
			case strings.HasPrefix(line, "author-time "):
				curTime, _ = strconv.ParseInt(line[len("author-time "):], 10, 64)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return FileBlame{}, err
		}
	}
	fb.Generated = HasGeneratedHeader(head)
	return fb, nil
}

// SortedAuthors returns the authors of a FileBlame by descending line count,
// ties broken by name for determinism.
func (fb FileBlame) SortedAuthors() []Author {
	as := make([]Author, 0, len(fb.Lines))
	for a := range fb.Lines {
		as = append(as, a)
	}
	sort.Slice(as, func(i, j int) bool {
		if fb.Lines[as[i]] != fb.Lines[as[j]] {
			return fb.Lines[as[i]] > fb.Lines[as[j]]
		}
		return as[i].Name < as[j].Name
	})
	return as
}
