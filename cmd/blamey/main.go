// Command blamey shows who wrote the code that exists today.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/adnair2024/blamey/internal/output"
	"github.com/adnair2024/blamey/internal/pipeline"
)

// listFlag collects a repeatable string flag.
type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

type config struct {
	pipeline.Options
	json, plain, bots bool
}

func main() { os.Exit(realMain()) }

// realMain returns the exit code so deferred cleanup runs before exiting.
func realMain() int {
	cfg, err := parseArgs(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "blamey:", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, cfg, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "blamey:", err)
		return 1
	}
	return 0
}

// parseArgs accepts flags before or after the single optional repo path, which
// the standard flag package would otherwise stop at.
func parseArgs(args []string, errOut io.Writer) (config, error) {
	var (
		cfg              config
		include, exclude listFlag
	)
	fs := flag.NewFlagSet("blamey", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintln(errOut, "usage: blamey [flags] [path]")
		fs.PrintDefaults()
	}
	fs.StringVar(&cfg.Rev, "rev", "HEAD", "revision to blame")
	fs.StringVar(&cfg.Since, "since", "", "only count history after this date (ownership is unaffected)")
	fs.StringVar(&cfg.Until, "until", "", "only count history before this date (ownership is unaffected)")
	fs.Var(&include, "include", "glob to count even if excluded by default (repeatable)")
	fs.Var(&exclude, "exclude", "glob to skip (repeatable)")
	fs.BoolVar(&cfg.IncludeMerges, "include-merges", false, "count merge commits")
	fs.BoolVar(&cfg.MergeByName, "merge-by-name", false, "merge authors that share a normalized name")
	fs.BoolVar(&cfg.json, "json", false, "print the full report as JSON and exit")
	fs.BoolVar(&cfg.plain, "plain", false, "print a text summary and exit")
	fs.BoolVar(&cfg.bots, "bots", false, "list bots in the plain summary")
	fs.IntVar(&cfg.Jobs, "jobs", 0, "parallel blame workers (default: number of CPUs)")

	var rest []string
	for {
		if err := fs.Parse(args); err != nil {
			return cfg, err
		}
		if fs.NArg() == 0 {
			break
		}
		rest = append(rest, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(rest) > 1 {
		return cfg, fmt.Errorf("expected at most one path, got %d", len(rest))
	}
	cfg.Repo = "."
	if len(rest) == 1 {
		cfg.Repo = rest[0]
	}
	cfg.Include, cfg.Exclude = include, exclude
	return cfg, nil
}

func run(ctx context.Context, cfg config, stdout, stderr *os.File) error {
	prog := newProgress(stderr)
	report, err := pipeline.Run(ctx, cfg.Options, prog.update)
	prog.clear()
	if err != nil {
		return err
	}
	if cfg.json {
		return output.JSON(stdout, report)
	}
	// The interactive TUI arrives in a later phase; until then every path ends
	// in the plain summary, which is also what non-TTY output must be.
	output.Plain(stdout, report, output.Options{ShowBots: cfg.bots})
	return nil
}

// progress prints phase updates to stderr, but only when it is a terminal so
// logs and pipes stay clean.
type progress struct {
	mu  sync.Mutex
	out *os.File
	tty bool
}

func newProgress(out *os.File) *progress {
	return &progress{out: out, tty: isTTY(out)}
}

func (p *progress) update(pr pipeline.Progress) {
	if !p.tty {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if pr.Total > 0 {
		fmt.Fprintf(p.out, "\r\x1b[K%s %s/%s files", pr.Phase, output.Num(pr.Done), output.Num(pr.Total))
	} else {
		fmt.Fprintf(p.out, "\r\x1b[K%s…", pr.Phase)
	}
}

func (p *progress) clear() {
	if p.tty {
		fmt.Fprint(p.out, "\r\x1b[K")
	}
}

func isTTY(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
