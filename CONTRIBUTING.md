# Contributing

Thanks for your interest in ibkrctl.

## Setup

```sh
git clone https://github.com/laurenschristian/ibkrctl.git
cd ibkrctl
make hooks        # run the same checks as CI on commit/push
go build -o /tmp/ibkrctl . && /tmp/ibkrctl --help
```

Requires Go 1.27 or later.

## Before opening a PR

```sh
make fmt          # gofmt
make lint         # golangci-lint (includes go vet)
make test         # go test -race -shuffle=on
make cover        # enforces the coverage floor
make docs-check   # man/ and docs/cli/ regenerate deterministically
```

CI runs all of these on every push and PR. New behavior needs a test; the fake
eero server in `internal/cli` and `internal/eero` makes that cheap, no network
required.

## House rules

- No em or en dashes in code, docs or commit messages. Use `:`, `,`, `;`, `(` or two sentences. The pre-commit hook enforces it.
- Conventional commit subjects (`feat:`, `fix:`, `docs:`, ...); the changelog is grouped from them.
- Keep the CLI and the MCP server in sync: a new operation should appear in both.
- Field names in `internal/eero` mirror the eero API verbatim, even when they read oddly.

## Layout

| Path | What |
|---|---|
| `internal/eero` | thin API client and response types |
| `internal/cli` | cobra commands and the MCP server |
| `internal/config` | config file, env and Keychain token handling |
| `tools/gendocs` | regenerates man pages and markdown from the command tree |

## Releasing

Tag `vX.Y.Z` on `main`; the release workflow runs goreleaser (binaries, checksums,
man pages, completions, Homebrew formula). See the tag history for the cadence.
