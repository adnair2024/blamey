package main

import (
	"io"
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	cfg, err := parseArgs([]string{"--json", "some/repo", "--since", "2024-01-01", "--include", "a", "--include", "b", "--jobs", "3"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Repo != "some/repo" || !cfg.json || cfg.Since != "2024-01-01" || cfg.Jobs != 3 || cfg.Rev != "HEAD" {
		t.Errorf("cfg = %+v", cfg)
	}
	if !reflect.DeepEqual(cfg.Include, []string{"a", "b"}) {
		t.Errorf("include = %v", cfg.Include)
	}
	if cfg, _ := parseArgs(nil, io.Discard); cfg.Repo != "." {
		t.Errorf("default repo = %q", cfg.Repo)
	}
	if _, err := parseArgs([]string{"a", "b"}, io.Discard); err == nil {
		t.Error("two paths should error")
	}
	if _, err := parseArgs([]string{"--bogus"}, io.Discard); err == nil {
		t.Error("unknown flag should error")
	}
}
