#!/bin/sh
# Emit shell completion scripts into completions/ (used by goreleaser + make docs).
set -e
mkdir -p completions
go run . completion bash > completions/ibkrctl.bash
go run . completion zsh  > completions/_ibkrctl
go run . completion fish > completions/ibkrctl.fish
