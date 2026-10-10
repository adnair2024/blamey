// Package cache stores parsed git results on disk so repeat runs only redo
// work for files that changed. Every failure is a cache miss: a missing,
// unreadable or corrupt cache must never turn into an error for the user.
package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"github.com/adnair2024/blamey/internal/git"
)

// version invalidates every entry when the stored shapes or the way results
// are produced change.
const version = 1

const (
	maxAge       = 30 * 24 * time.Hour // unused entries are pruned after this
	pruneEvery   = 24 * time.Hour
	keepLogFiles = 5
)

// Store is a per-repository cache directory. A nil *Store is valid and acts as
// a cache that never hits, so callers need no special case for --no-cache.
type Store struct {
	dir         string
	fingerprint string
	hits        atomic.Int64
	misses      atomic.Int64
}

// Open returns the cache for the repository at repo (an absolute path). base is
// the cache root; empty means $XDG_CACHE_HOME or the OS default. It returns
// nil, never an error, when no usable directory exists.
func Open(repo, base string) *Store {
	if base == "" {
		var err error
		if base, err = os.UserCacheDir(); err != nil {
			return nil
		}
	}
	sum := sha256.Sum256([]byte(repo))
	dir := filepath.Join(base, "blamey", hex.EncodeToString(sum[:8]))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	s := &Store{dir: dir, fingerprint: fingerprint(repo)}
	s.prune()
	return s
}

// fingerprint covers inputs that change blame and log output without changing
// a blob or commit: the mailmap and the ignore-revs list.
func fingerprint(repo string) string {
	h := sha256.New()
	h.Write([]byte{version})
	for _, name := range []string{".mailmap", ".git-blame-ignore-revs"} {
		b, _ := os.ReadFile(filepath.Join(repo, name))
		h.Write([]byte(name))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Stats reports lookups since Open.
func (s *Store) Stats() (hits, misses int64) {
	if s == nil {
		return 0, 0
	}
	return s.hits.Load(), s.misses.Load()
}

func (s *Store) key(parts ...string) string {
	h := sha256.New()
	h.Write([]byte(s.fingerprint))
	for _, p := range parts {
		h.Write([]byte{0})
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// entry wraps a payload with the key it was stored under, so a stray or
// colliding file is rejected instead of trusted.
type entry[T any] struct {
	Key     string
	Payload T
}

func read[T any](s *Store, file, key string) (T, bool) {
	var zero T
	f, err := os.Open(file)
	if err != nil {
		s.misses.Add(1)
		return zero, false
	}
	defer f.Close()
	var e entry[T]
	if err := gob.NewDecoder(f).Decode(&e); err != nil || e.Key != key {
		s.misses.Add(1)
		return zero, false
	}
	s.hits.Add(1)
	now := time.Now()
	_ = os.Chtimes(file, now, now) // keep used entries from being pruned
	return e.Payload, true
}

// write stores atomically: readers never see a partial file.
func write[T any](file, key string, payload T) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(entry[T]{Key: key, Payload: payload}); err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".tmp-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(buf.Bytes())
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), file) != nil {
		_ = os.Remove(tmp.Name())
	}
}

func (s *Store) blamePath(key string) string {
	return filepath.Join(s.dir, "blame", key[:2], key+".gob")
}

// Get implements git.BlameCache, keyed by (path, blob sha).
func (s *Store) Get(path, blob string) (git.FileBlame, bool) {
	if s == nil {
		return git.FileBlame{}, false
	}
	k := s.key("blame", path, blob)
	return read[git.FileBlame](s, s.blamePath(k), k)
}

// Put implements git.BlameCache.
func (s *Store) Put(path, blob string, fb git.FileBlame) {
	if s == nil {
		return
	}
	k := s.key("blame", path, blob)
	write(s.blamePath(k), k, fb)
}

// LogKey identifies a parsed history: the exact commit plus every option that
// changes which commits are listed.
type LogKey struct {
	Head          string
	Rev           string
	Since, Until  string
	IncludeMerges bool
}

func (s *Store) logPath(k LogKey) (path, key string) {
	merges := "0"
	if k.IncludeMerges {
		merges = "1"
	}
	key = s.key("log", k.Head, k.Rev, k.Since, k.Until, merges)
	return filepath.Join(s.dir, "log-"+key+".gob"), key
}

// GetLog returns the cached history for k.
func (s *Store) GetLog(k LogKey) ([]git.Commit, bool) {
	if s == nil {
		return nil, false
	}
	p, key := s.logPath(k)
	return read[[]git.Commit](s, p, key)
}

// PutLog stores the history for k and drops older history files, since each
// new HEAD makes the previous ones unlikely to be reused.
func (s *Store) PutLog(k LogKey, commits []git.Commit) {
	if s == nil {
		return
	}
	p, key := s.logPath(k)
	write(p, key, commits)
	s.trimLogs()
}

func (s *Store) trimLogs() {
	files, _ := filepath.Glob(filepath.Join(s.dir, "log-*.gob"))
	if len(files) <= keepLogFiles {
		return
	}
	type f struct {
		path string
		mod  time.Time
	}
	var fs []f
	for _, p := range files {
		if st, err := os.Stat(p); err == nil {
			fs = append(fs, f{p, st.ModTime()})
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].mod.After(fs[j].mod) })
	for _, old := range fs[min(keepLogFiles, len(fs)):] {
		_ = os.Remove(old.path)
	}
}

// prune removes blame entries untouched for maxAge, at most once per
// pruneEvery, so the cache cannot grow without bound.
func (s *Store) prune() {
	marker := filepath.Join(s.dir, ".pruned")
	if st, err := os.Stat(marker); err == nil && time.Since(st.ModTime()) < pruneEvery {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	_ = filepath.WalkDir(filepath.Join(s.dir, "blame"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // best effort
		}
		if info, err := d.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(p)
		}
		return nil
	})
	_ = os.WriteFile(marker, nil, 0o644)
}
