package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// gitCmd builds a git invocation scoped to repo with -C, never chdir.
func gitCmd(ctx context.Context, repo string, args ...string) *exec.Cmd {
	full := append([]string{"-C", repo}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	// Keep output stable regardless of the user's locale or pager.
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "GIT_PAGER=cat")
	return cmd
}

// runGit runs git to completion and returns stdout. Stderr is included in the
// error so callers see why git refused.
func runGit(ctx context.Context, repo string, stdin []byte, args ...string) ([]byte, error) {
	cmd := gitCmd(ctx, repo, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// HasCommits reports whether rev resolves to a commit. A fresh repo with no
// commits returns false without error.
func HasCommits(ctx context.Context, repo, rev string) bool {
	_, err := runGit(ctx, repo, nil, "rev-parse", "--verify", "-q", rev+"^{commit}")
	return err == nil
}

// IsShallow reports whether the repo is a shallow clone, whose history is
// incomplete.
func IsShallow(ctx context.Context, repo string) (bool, error) {
	out, err := runGit(ctx, repo, nil, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// HeadSHA resolves rev to a full commit sha; used as the cache key.
func HeadSHA(ctx context.Context, repo, rev string) (string, error) {
	out, err := runGit(ctx, repo, nil, "rev-parse", "--verify", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
