// Package pipeline wires the git layer to the stats layer. It is the only place
// that knows both, so tui stays free of git and stats stays free of I/O.
package pipeline

import (
	"context"
	"fmt"
	"path/filepath"

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
	Cache         git.BlameCache // optional
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

	progress(Progress{Phase: "reading history"})
	commits, err := git.Log(ctx, abs, git.LogOptions{
		Rev: opts.Rev, Since: opts.Since, Until: opts.Until, IncludeMerges: opts.IncludeMerges,
	})
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
		Rev: opts.Rev, Jobs: opts.Jobs, Cache: opts.Cache, Filter: filter,
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
