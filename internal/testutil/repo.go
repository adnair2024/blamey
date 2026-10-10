// Package testutil builds real git repositories with scripted history so tests
// have exact, known expected numbers.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Repo is a throwaway repository in t.TempDir().
type Repo struct {
	t   *testing.T
	Dir string
}

// NewRepo creates an empty repo on branch main. The developer's global git
// config is disabled so it cannot leak into results.
func NewRepo(t *testing.T) *Repo {
	t.Helper()
	r := &Repo{t: t, Dir: t.TempDir()}
	r.Git(nil, "init", "-q", "-b", "main")
	return r
}

func (r *Repo) env(author, email string, when time.Time) []string {
	env := append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
		"LC_ALL=C",
	)
	if author != "" {
		ts := when.Format("2006-01-02T15:04:05-07:00")
		env = append(env,
			"GIT_AUTHOR_NAME="+author, "GIT_AUTHOR_EMAIL="+email, "GIT_AUTHOR_DATE="+ts,
			"GIT_COMMITTER_NAME="+author, "GIT_COMMITTER_EMAIL="+email, "GIT_COMMITTER_DATE="+ts,
		)
	}
	return env
}

// Git runs git in the repo and fails the test on error. extra overrides env.
func (r *Repo) Git(extra []string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.Dir}, args...)...)
	cmd.Env = append(r.env("T", "t@example.com", time.Unix(0, 0).UTC()), extra...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// Write creates or replaces a file (creating directories as needed).
func (r *Repo) Write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.Dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// Commit stages everything and commits as the given author at the given time.
// allow-empty so scripts can model empty commits.
func (r *Repo) Commit(author, email string, when time.Time, msg string) string {
	r.t.Helper()
	r.Git(nil, "add", "-A")
	r.Git(r.env(author, email, when), "commit", "-q", "--allow-empty", "--no-gpg-sign", "-m", msg)
	return strings.TrimSpace(r.Git(nil, "rev-parse", "HEAD"))
}

// Lines returns n lines "<prefix>1\n<prefix>2\n...", handy for exact counts.
func Lines(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString(prefix)
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	return b.String()
}
