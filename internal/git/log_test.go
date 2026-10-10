package git

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adnair2024/blamey/internal/testutil"
)

// rec builds raw `git log -z` output for one commit.
func rec(hash, name, email, unix, tz, subject, numstat string) string {
	h := strings.Join([]string{hash, name, email, unix, tz, name, subject}, "\x1f")
	return "\x1e" + h + "\x00" + numstat
}

func TestParseLog(t *testing.T) {
	raw := rec("aaa", "Zoë 李", "z@x", "1700000000", "+0530", "unicode | pipe\tand tab",
		"\n3\t1\tsrc/a.go\x00-\t-\timg/logo.png\x00") +
		rec("bbb", "A|B\tC", "ab@x", "1600000000", "-0500", "rename",
			"\n2\t2\t\x00old/name.go\x00new/name.go\x00") +
		rec("ccc", "Empty", "e@x", "1500000000", "+0000", "empty", "")

	got, err := ParseLog(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 commits, got %d", len(got))
	}

	c := got[0]
	if c.AuthorName != "Zoë 李" || c.Subject != "unicode | pipe\tand tab" {
		t.Errorf("unicode/pipe fields mangled: %+v", c)
	}
	if _, off := c.Time.Zone(); off != 5*3600+1800 || c.Time.Unix() != 1700000000 {
		t.Errorf("time/zone wrong: %v", c.Time)
	}
	wantFiles := []FileChange{
		{Path: "src/a.go", Added: 3, Deleted: 1},
		{Path: "img/logo.png", Binary: true},
	}
	if len(c.Files) != 2 || c.Files[0] != wantFiles[0] || c.Files[1] != wantFiles[1] {
		t.Errorf("files = %+v", c.Files)
	}

	if got[1].AuthorName != "A|B\tC" {
		t.Errorf("author with pipe/tab: %q", got[1].AuthorName)
	}
	if f := got[1].Files; len(f) != 1 || f[0].Path != "new/name.go" || f[0].OldPath != "old/name.go" || f[0].Added != 2 {
		t.Errorf("rename = %+v", f)
	}
	if len(got[2].Files) != 0 {
		t.Errorf("empty commit has files: %+v", got[2].Files)
	}
}

func TestParseLogEmptyAndMalformed(t *testing.T) {
	if got, err := ParseLog(strings.NewReader("")); err != nil || len(got) != 0 {
		t.Errorf("empty input: %v %v", got, err)
	}
	if _, err := ParseLog(strings.NewReader("\x1enot-a-header\x00")); err == nil {
		t.Error("expected error for malformed header")
	}
}

func TestResolveRename(t *testing.T) {
	tests := []struct{ in, old, new string }{
		{"a.go => b.go", "a.go", "b.go"},
		{"dir/{a => b}/c.go", "dir/a/c.go", "dir/b/c.go"},
		{"dir/{ => sub}/c.go", "dir/c.go", "dir/sub/c.go"},
		{"dir/{sub => }/c.go", "dir/sub/c.go", "dir/c.go"},
		{"plain.go", "plain.go", "plain.go"},
	}
	for _, tt := range tests {
		o, n := ResolveRename(tt.in)
		if o != tt.old || n != tt.new {
			t.Errorf("%q => %q, %q; want %q, %q", tt.in, o, n, tt.old, tt.new)
		}
	}
}

func day(d int) time.Time {
	return time.Date(2024, 1, d, 12, 0, 0, 0, time.FixedZone("", 2*3600))
}

func TestLogIntegration(t *testing.T) {
	ctx := context.Background()
	r := testutil.NewRepo(t)

	if cs, err := Log(ctx, r.Dir, LogOptions{}); err != nil || len(cs) != 0 {
		t.Fatalf("empty repo: %v %v", cs, err)
	}

	r.Write("a.txt", testutil.Lines("a", 3))
	r.Commit("Alice", "alice@example.com", day(1), "first")
	r.Write("a.txt", testutil.Lines("a", 3)+"extra\n")
	r.Write("b.txt", testutil.Lines("b", 5))
	r.Commit("Bob", "bob@example.com", day(2), "second")
	r.Commit("Bob", "bob@example.com", day(3), "empty")

	cs, err := Log(ctx, r.Dir, LogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 3 {
		t.Fatalf("want 3 commits, got %d", len(cs))
	}
	if cs[0].Subject != "empty" || len(cs[0].Files) != 0 {
		t.Errorf("newest = %+v", cs[0])
	}
	second := cs[1]
	if second.AuthorName != "Bob" || len(second.Files) != 2 {
		t.Fatalf("second = %+v", second)
	}
	added := 0
	for _, f := range second.Files {
		added += f.Added
	}
	if added != 6 {
		t.Errorf("added = %d, want 6", added)
	}
	if _, off := second.Time.Zone(); off != 2*3600 {
		t.Errorf("author tz offset = %d, want 7200", off)
	}

	since, err := Log(ctx, r.Dir, LogOptions{Since: "2024-01-03 00:00:00 +0200"})
	if err != nil || len(since) != 1 {
		t.Errorf("since filter: %d commits, err %v", len(since), err)
	}
}

func TestLogMailmapAndMerges(t *testing.T) {
	ctx := context.Background()
	r := testutil.NewRepo(t)
	r.Write(".mailmap", "Alice Real <alice@example.com> <old@example.com>\n")
	r.Write("a.txt", "x\n")
	r.Commit("alice-old", "old@example.com", day(1), "base")
	r.Git(nil, "checkout", "-q", "-b", "feat")
	r.Write("f.txt", "f\n")
	r.Commit("Carol", "carol@example.com", day(2), "feat")
	r.Git(nil, "checkout", "-q", "main")
	r.Write("m.txt", "m\n")
	r.Commit("Bob", "bob@example.com", day(3), "main work")
	r.Git(testutilEnv(), "merge", "-q", "--no-ff", "--no-gpg-sign", "-m", "merge", "feat")

	cs, _ := Log(ctx, r.Dir, LogOptions{})
	if len(cs) != 3 {
		t.Fatalf("no-merges: want 3, got %d", len(cs))
	}
	if cs[2].AuthorName != "Alice Real" {
		t.Errorf("mailmap not applied: %q", cs[2].AuthorName)
	}
	all, _ := Log(ctx, r.Dir, LogOptions{IncludeMerges: true})
	if len(all) != 4 {
		t.Errorf("include-merges: want 4, got %d", len(all))
	}
}

func testutilEnv() []string {
	return []string{"GIT_AUTHOR_NAME=M", "GIT_AUTHOR_EMAIL=m@x", "GIT_COMMITTER_NAME=M", "GIT_COMMITTER_EMAIL=m@x"}
}
