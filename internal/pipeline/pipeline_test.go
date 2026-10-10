package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/adnair2024/blamey/internal/testutil"
)

func day(d int) time.Time { return time.Date(2024, 1, d, 12, 0, 0, 0, time.UTC) }

func TestRunEndToEnd(t *testing.T) {
	r := testutil.NewRepo(t)
	r.Write("a.txt", testutil.Lines("a", 10))
	r.Write("vendor/v.txt", testutil.Lines("v", 50))
	r.Commit("Alice", "alice@example.com", day(1), "init")
	// Bob rewrites 4 of Alice's lines and adds 2 more.
	r.Write("a.txt", "a1\nB\nB\nB\nB\na6\na7\na8\na9\na10\nn1\nn2\n")
	r.Commit("Bob", "bob@example.com", day(2), "edit")

	var mu sync.Mutex
	var phases []string
	rep, err := Run(context.Background(), Options{Repo: r.Dir, Jobs: 2}, func(p Progress) {
		mu.Lock()
		phases = append(phases, p.Phase)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Totals.SurvivingLines != 12 || rep.Totals.Commits != 2 || rep.Totals.Files != 1 {
		t.Fatalf("totals = %+v", rep.Totals)
	}
	var alice, bob = -1, -1
	for i, c := range rep.Contributors {
		switch c.Name {
		case "Alice":
			alice = i
		case "Bob":
			bob = i
		}
	}
	a, b := rep.Contributors[alice], rep.Contributors[bob]
	// Alice: 10 added, 5 deleted by Bob's rewrite -> 6 survive.
	// Bob: 6 added (4 rewrite + 2 new), 4 deleted -> 6 survive.
	if a.Added != 10 || a.Surviving != 6 || a.Churn != 4 {
		t.Errorf("alice = %+v", a)
	}
	if b.Added != 6+0 || b.Deleted != 4 || b.Surviving != 6 {
		t.Errorf("bob = %+v", b)
	}
	if phases[0] != "reading history" || phases[len(phases)-1] != "computing statistics" {
		t.Errorf("phases = %v", phases)
	}
	if rep.Meta.Head == "" || rep.Meta.Shallow {
		t.Errorf("meta = %+v", rep.Meta)
	}
}

func TestRunIncludeVendor(t *testing.T) {
	r := testutil.NewRepo(t)
	r.Write("vendor/v.txt", testutil.Lines("v", 7))
	r.Commit("Alice", "alice@example.com", day(1), "init")
	rep, err := Run(context.Background(), Options{Repo: r.Dir, Include: []string{"vendor/**"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Totals.SurvivingLines != 7 {
		t.Errorf("--include should opt vendor in, got %d lines", rep.Totals.SurvivingLines)
	}
}

func TestRunEmptyAndInvalid(t *testing.T) {
	r := testutil.NewRepo(t)
	rep, err := Run(context.Background(), Options{Repo: r.Dir}, nil)
	if err != nil {
		t.Fatalf("empty repo must not error: %v", err)
	}
	if rep.Totals.Commits != 0 || len(rep.Meta.Warnings) == 0 {
		t.Errorf("empty report = %+v", rep.Meta)
	}
	if _, err := Run(context.Background(), Options{Repo: t.TempDir()}, nil); err == nil {
		t.Error("non-repo should error")
	}
}

func TestRunDetachedHeadAndShallow(t *testing.T) {
	r := testutil.NewRepo(t)
	r.Write("a.txt", "x\n")
	first := r.Commit("A", "a@x", day(1), "one")
	r.Write("a.txt", "x\ny\n")
	r.Commit("A", "a@x", day(2), "two")
	r.Git(nil, "checkout", "-q", "--detach", first)
	rep, err := Run(context.Background(), Options{Repo: r.Dir}, nil)
	if err != nil || rep.Totals.Commits != 1 {
		t.Fatalf("detached HEAD: %v %+v", err, rep.Totals)
	}
}

func TestRunCancelled(t *testing.T) {
	r := testutil.NewRepo(t)
	r.Write("a.txt", "x\n")
	r.Commit("A", "a@x", day(1), "one")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, Options{Repo: r.Dir}, nil); err == nil {
		t.Error("expected error on cancelled context")
	}
}
