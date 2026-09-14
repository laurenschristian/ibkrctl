# ibkrctl

[![CI](https://github.com/laurenschristian/ibkrctl/actions/workflows/ci.yml/badge.svg)](https://github.com/laurenschristian/ibkrctl/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

CLI and MCP server for Interactive Brokers One static binary: a CLI and an [MCP](https://modelcontextprotocol.io) server for the IBKR Client Portal Gateway.

> Status: scaffold. The client, commands and MCP tools are being built. `ibkrctl doctor` and `ibkrctl mcp` work today.

## Install

```console
brew install laurenschristian/tap/ibkrctl
go install github.com/laurenschristian/ibkrctl@latest
```

## Configure

Precedence is flags, then environment (`IBKR_URL`, `IBKR_USER`, `IBKR_PASS`, `IBKR_CONFIG`), then the config file
(`~/Library/Application Support/ibkrctl/config.yaml` on macOS, `~/.config/ibkrctl/config.yaml` on Linux).
`password_cmd` runs any command that prints the secret, so it can live in a keychain, `op read`, `pass` or sops.

## MCP

```console
claude mcp add ibkr -- ibkrctl mcp
```

## Development

```console
make hooks    # gofmt, dash check, gitleaks, build, lint on commit; tests + coverage floor on push
make test
make lint
make docs     # regenerate man/ and docs/cli/
```

See [CONTRIBUTING.md](CONTRIBUTING.md). MIT.
