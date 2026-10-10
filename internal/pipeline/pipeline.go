// Package pipeline wires the git layer to the stats layer. It is the only place
// that knows both, so tui stays free of git and stats stays free of I/O.
package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/adnair2024/blamey/internal/cache"
	"github.com/adnair2024/blamey/internal/git"
	"github.com/adnair2024/blamey/internal/stats"
)

// hugeRepoFiles is the file count above which we warn that blaming everything
// will be slow.
const hugeRepoFiles = 20000

// Options are the user-facing settings that shape a run.
type Options struct {
	Repo          string
	Rev           string
	Since, Until  string
	Include       []string
	Exclude       []string
	IncludeMerges bool
	MergeByName   bool
	Jobs          int
	// NoCache bypasses the on-disk cache (--no-cache).
	NoCache bool
	// CacheDir overrides the cache root; empty means the user cache dir.
	CacheDir string
}

// Progress describes the current phase. Total is 0 when the phase has no
// countable steps.
type Progress struct {
	Phase       string
	Done, Total int
}

// Run collects history and blame and returns the Report. progress may be nil;
// it is called from several goroutines, so receivers must be cheap and safe.
func Run(ctx context.Context, opts Options, progress func(Progress)) (stats.Report, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	if opts.Repo == "" {
		opts.Repo = "."
	}
	if opts.Rev == "" {
		opts.Rev = "HEAD"
	}
	abs, err := filepath.Abs(opts.Repo)
	if err != nil {
		return stats.Report{}, fmt.Errorf("resolve %s: %w", opts.Repo, err)
	}

	meta := stats.Meta{Path: abs, Rev: opts.Rev, Since: opts.Since, Until: opts.Until}
	if shallow, err := git.IsShallow(ctx, abs); err != nil {
		return stats.Report{}, fmt.Errorf("%s is not a git repository: %w", abs, err)
	} else if shallow {
		meta.Shallow = true
		meta.Warnings = append(meta.Warnings, "shallow clone: history is incomplete")
	}
	if !git.HasCommits(ctx, abs, opts.Rev) {
		meta.Warnings = append(meta.Warnings, "no commits at "+opts.Rev)
		return stats.Build(stats.Input{Meta: meta}), nil
	}
	if meta.Head, err = git.HeadSHA(ctx, abs, opts.Rev); err != nil {
		return stats.Report{}, err
	}

	var store *cache.Store
	var blameCache git.BlameCache
	if !opts.NoCache {
		if store = cache.Open(abs, opts.CacheDir); store != nil {
			blameCache = store
		}
	}

	progress(Progress{Phase: "reading history"})
	commits, err := loadHistory(ctx, abs, meta.Head, store, opts)
	if err != nil {
		return stats.Report{}, err
	}

	filter := git.NewFilter(opts.Include, opts.Exclude)
	progress(Progress{Phase: "listing files"})
	files, err := git.ListFiles(ctx, abs, opts.Rev, filter)
	if err != nil {
		return stats.Report{}, err
	}
	if len(files) > hugeRepoFiles {
		meta.Warnings = append(meta.Warnings, fmt.Sprintf(
			"%d files to blame; this will be slow. Scope it with --include", len(files)))
	}

	progress(Progress{Phase: "blaming", Total: len(files)})
	blame, err := git.BlameAll(ctx, abs, files, git.BlameOptions{
		Rev: opts.Rev, Jobs: opts.Jobs, Cache: blameCache, Filter: filter,
		Progress: func(done, total int) {
			progress(Progress{Phase: "blaming", Done: done, Total: total})
		},
	})
	if err != nil {
		return stats.Report{}, err
	}

	progress(Progress{Phase: "computing statistics"})
	return stats.Build(stats.Input{
		Meta: meta, Commits: commits, Blame: blame, Filter: filter, MergeByName: opts.MergeByName,
	}), nil
}

// loadHistory reads the log, using the cache when the same HEAD and options
// were parsed before.
func loadHistory(ctx context.Context, repo, head string, store *cache.Store, opts Options) ([]git.Commit, error) {
	key := cache.LogKey{Head: head, Rev: opts.Rev, Since: opts.Since, Until: opts.Until, IncludeMerges: opts.IncludeMerges}
	// A relative window like "2 weeks ago" means something different tomorrow,
	// so only absolute dates are safe to cache against a fixed HEAD.
	cacheable := absoluteDate(opts.Since) && absoluteDate(opts.Until)
	if cacheable {
		if commits, ok := store.GetLog(key); ok {
			return commits, nil
		}
	}
	commits, err := git.Log(ctx, repo, git.LogOptions{
		Rev: opts.Rev, Since: opts.Since, Until: opts.Until, IncludeMerges: opts.IncludeMerges,
	})
	if err != nil {
		return nil, err
	}
	if cacheable {
		store.PutLog(key, commits)
	}
	return commits, nil
}

func absoluteDate(s string) bool {
	if s == "" {
		return true
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", time.RFC3339} {
		if _, err := time.Parse(layout, s); err == nil {
			return true
		}
	}
	return false
}
