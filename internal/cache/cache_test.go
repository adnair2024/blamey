package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adnair2024/blamey/internal/git"
)

func sample() git.FileBlame {
	return git.FileBlame{
		Path: "a.go", Total: 3, LastTime: 99,
		Lines: map[git.Author]int{{Name: "李雷", Email: "l@x"}: 2, {Name: "Bob", Email: "b@x"}: 1},
	}
}

func TestBlameRoundTripAndKeying(t *testing.T) {
	repo := t.TempDir()
	s := Open(repo, t.TempDir())
	if _, ok := s.Get("a.go", "blob1"); ok {
		t.Fatal("empty cache hit")
	}
	s.Put("a.go", "blob1", sample())
	got, ok := s.Get("a.go", "blob1")
	if !ok || got.Total != 3 || got.Lines[git.Author{Name: "李雷", Email: "l@x"}] != 2 || got.LastTime != 99 {
		t.Fatalf("round trip = %+v ok=%v", got, ok)
	}
	if _, ok := s.Get("a.go", "blob2"); ok {
		t.Error("different blob must miss")
	}
	if _, ok := s.Get("b.go", "blob1"); ok {
		t.Error("different path must miss")
	}
	if h, m := s.Stats(); h != 1 || m != 3 {
		t.Errorf("stats hits=%d misses=%d", h, m)
	}
}

func TestCorruptEntryIsAMiss(t *testing.T) {
	s := Open(t.TempDir(), t.TempDir())
	s.Put("a.go", "b", sample())
	k := s.key("blame", "a.go", "b")
	if err := os.WriteFile(s.blamePath(k), []byte("not gob at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("a.go", "b"); ok {
		t.Error("corrupt entry must miss, not error or hit")
	}
	// Truncated file.
	s.Put("a.go", "c", sample())
	p := s.blamePath(s.key("blame", "a.go", "c"))
	data, _ := os.ReadFile(p)
	_ = os.WriteFile(p, data[:len(data)/2], 0o644)
	if _, ok := s.Get("a.go", "c"); ok {
		t.Error("truncated entry must miss")
	}
}

func TestEntryForWrongKeyIsRejected(t *testing.T) {
	s := Open(t.TempDir(), t.TempDir())
	s.Put("a.go", "one", sample())
	src := s.blamePath(s.key("blame", "a.go", "one"))
	dst := s.blamePath(s.key("blame", "a.go", "two"))
	data, _ := os.ReadFile(src)
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	_ = os.WriteFile(dst, data, 0o644)
	if _, ok := s.Get("a.go", "two"); ok {
		t.Error("a file stored under another key must not be trusted")
	}
}

func TestFingerprintInvalidates(t *testing.T) {
	repo, base := t.TempDir(), t.TempDir()
	s1 := Open(repo, base)
	s1.Put("a.go", "b", sample())
	if err := os.WriteFile(filepath.Join(repo, ".mailmap"), []byte("New <n@x> <old@x>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := Open(repo, base)
	if _, ok := s2.Get("a.go", "b"); ok {
		t.Error("changing .mailmap must invalidate blame results")
	}
	s3 := Open(repo, base)
	s3.Put("a.go", "b", sample())
	if err := os.WriteFile(filepath.Join(repo, ".git-blame-ignore-revs"), []byte("abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := Open(repo, base).Get("a.go", "b"); ok {
		t.Error("changing ignore-revs must invalidate blame results")
	}
}

func TestLogKeying(t *testing.T) {
	s := Open(t.TempDir(), t.TempDir())
	k := LogKey{Head: "h1", Rev: "HEAD"}
	commits := []git.Commit{{Hash: "x", AuthorName: "A", Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.FixedZone("", 3600)),
		Files: []git.FileChange{{Path: "p", Added: 1}}}}
	s.PutLog(k, commits)
	got, ok := s.GetLog(k)
	if !ok || len(got) != 1 || got[0].Files[0].Path != "p" {
		t.Fatalf("log round trip: %+v %v", got, ok)
	}
	if _, off := got[0].Time.Zone(); off != 3600 || !got[0].Time.Equal(commits[0].Time) {
		t.Errorf("time not preserved: %v", got[0].Time)
	}
	for name, other := range map[string]LogKey{
		"head": {Head: "h2", Rev: "HEAD"}, "since": {Head: "h1", Rev: "HEAD", Since: "2024-01-01"},
		"until": {Head: "h1", Rev: "HEAD", Until: "2024-01-01"}, "merges": {Head: "h1", Rev: "HEAD", IncludeMerges: true},
		"rev": {Head: "h1", Rev: "main"},
	} {
		if _, ok := s.GetLog(other); ok {
			t.Errorf("%s change must miss", name)
		}
	}
}

func TestOldLogFilesAreTrimmed(t *testing.T) {
	s := Open(t.TempDir(), t.TempDir())
	for i := 0; i < keepLogFiles+3; i++ {
		s.PutLog(LogKey{Head: string(rune('a' + i))}, nil)
		time.Sleep(5 * time.Millisecond)
	}
	files, _ := filepath.Glob(filepath.Join(s.dir, "log-*.gob"))
	if len(files) != keepLogFiles {
		t.Errorf("kept %d log files, want %d", len(files), keepLogFiles)
	}
}

func TestPruneRemovesStaleEntriesOnly(t *testing.T) {
	repo, base := t.TempDir(), t.TempDir()
	s := Open(repo, base)
	s.Put("old.go", "b", sample())
	s.Put("new.go", "b", sample())
	oldPath := s.blamePath(s.key("blame", "old.go", "b"))
	stale := time.Now().Add(-maxAge - time.Hour)
	if err := os.Chtimes(oldPath, stale, stale); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(s.dir, ".pruned")) // force a prune on the next Open
	s2 := Open(repo, base)
	if _, ok := s2.Get("old.go", "b"); ok {
		t.Error("stale entry should have been pruned")
	}
	if _, ok := s2.Get("new.go", "b"); !ok {
		t.Error("fresh entry must survive pruning")
	}
}

func TestNilAndUnwritableStoresAreSafe(t *testing.T) {
	var s *Store
	s.Put("a", "b", sample())
	s.PutLog(LogKey{}, nil)
	if _, ok := s.Get("a", "b"); ok {
		t.Error("nil store hit")
	}
	if _, ok := s.GetLog(LogKey{}); ok {
		t.Error("nil store log hit")
	}
	if h, m := s.Stats(); h != 0 || m != 0 {
		t.Error("nil stats")
	}

	// A cache root that is a regular file cannot hold a directory.
	file := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(file, nil, 0o644)
	if Open(t.TempDir(), file) != nil {
		t.Error("unusable root should yield a nil store, not a panic")
	}
}
