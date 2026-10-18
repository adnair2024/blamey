# blamey

See who wrote the code that exists today.

`blamey` reads a git repository and shows how much of the current codebase each contributor actually owns, not just who made the most commits. It runs as an interactive terminal app, or prints a plain or JSON summary for CI and scripts.

## Why

Commit counts and lines added reward volume. A contributor who wrote 10,000 lines that were later rewritten looks bigger than one who wrote 2,000 lines that are still running in production. `blamey` uses `git blame` on the files at `HEAD` to count the lines that survive, then lays that next to the history so you can see both.

It answers:

- Who wrote the code that exists today, and what percentage of the codebase is theirs?
- Who added or removed the most over the history?
- Who is active now, and who has gone quiet?
- Which files and directories depend on a single person (bus factor)?

## Install

You need `git` on your `PATH` and a recent Go toolchain (see `go.mod`).

```sh
go install github.com/adnair2024/blamey/cmd/blamey@latest
```

Or build from source:

```sh
git clone https://github.com/adnair2024/blamey
cd blamey
go build -o bin/blamey ./cmd/blamey
```

## Usage

```sh
blamey                 # analyze the repo in the current directory
blamey path/to/repo    # analyze another repo
blamey --plain         # text summary, no TUI
blamey --json          # full report as JSON
```

When stdout is not a terminal (pipes, CI), `blamey` falls back to `--plain` on its own.

Example `--plain` output:

```
3,711 surviving lines in 27 files · 2 commits · 1 contributors (0 bots)
bus factor 1 · gini 0.00 · top 1 own 100.0% · top 3 own 100.0%

CONTRIBUTOR  COMMITS  +LINES  -LINES  SURVIVING    OWN%  ~CHURN  LAST ACTIVE
Ashwin Nair        2   3,712       1      3,711  100.0%      ~1  6 days ago (2026-10-11)

~ marks approximations (churn). Line counts measure volume, not value.
```

## The interface

Five tabs, switch with `tab` / `shift+tab` or `1`-`5`:

| Tab | Shows |
|---|---|
| Overview | totals, bus factor, top ownership bars, activity sparkline |
| Contributors | sortable table; `enter` opens a per-person detail view |
| Ownership | directory tree with ownership bars; drill in with `enter` |
| Activity | commits over time and a contribution-style heatmap |
| Files | per-file top owner and share; single-owner files are marked |

| Key | Action |
|---|---|
| `j` / `k`, arrows | move |
| `enter` / `esc` | open / back |
| `backspace` | up a directory level |
| `s` / `S` | cycle sort column / reverse it |
| `/` | filter |
| `b` | show or hide bots |
| `?` | help |
| `q`, `ctrl+c` | quit |

## Flags

| Flag | Meaning |
|---|---|
| `--rev <rev>` | revision to blame (default `HEAD`) |
| `--since <date>` | only count history after this date |
| `--until <date>` | only count history before this date |
| `--include <glob>` | count a path even if excluded by default (repeatable) |
| `--exclude <glob>` | skip a path (repeatable) |
| `--include-merges` | count merge commits |
| `--merge-by-name` | merge authors that share a normalized name |
| `--bots` | list bots in the plain summary |
| `--no-cache` | ignore and do not write the on-disk cache |
| `--jobs <n>` | parallel blame workers (default: number of CPUs) |
| `--plain` | print a text summary and exit |
| `--json` | print the full report as JSON and exit |

`--since` and `--until` limit the history numbers only. Ownership is always measured at the chosen revision.

## How it counts

- **Ownership** is the share of surviving lines at `HEAD` that `git blame` attributes to each person. It runs with `-w -M -C`, so whitespace changes and moved or copied lines don't take authorship away from the original author. A `.git-blame-ignore-revs` file in the repo root is honored.
- **Identities are merged** using `.mailmap`, matching emails (case-insensitive) and GitHub `noreply` addresses. Name-only matching is opt-in with `--merge-by-name`, because common names collide.
- **Bots** such as `dependabot[bot]` are tagged and hidden by default.
- **Skipped by default:** `vendor/`, `node_modules/`, `third_party/`, `dist/`, `build/`, lockfiles, generated files, and binaries. Use `--include` to opt back in.
- **Bus factor** is the smallest number of contributors whose combined ownership passes 50% of surviving lines.
- Numbers marked `~` are approximations, churn in particular.

Line counts measure volume, not value. A one-line fix can matter more than a thousand lines of scaffolding.

## Caching

Parsed history and blame results are cached per repository under your user cache directory (`$XDG_CACHE_HOME/blamey` on Linux), keyed by commit and blob hashes, so a second run on an unchanged repo is fast. A missing or corrupt cache is rebuilt silently. `--no-cache` bypasses it.

## Limits

- Shallow clones have incomplete history, and `blamey` warns when it detects one.
- Squash-merged work is credited to whoever authored the squash commit.
- Very large repositories take a while on the first run, since every file has to be blamed once.

## Development

```sh
go test ./...
go test -race ./...
golangci-lint run
```

## License

MIT, see [LICENSE](LICENSE).
