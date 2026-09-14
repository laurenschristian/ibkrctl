BIN := ibkrctl
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOLANGCI := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
GOSEC := github.com/securego/gosec/v2/cmd/gosec@v2.29.0
# G204: token_cmd is run through sh by design (user-supplied command, like adgctl's password_cmd).
GOSEC_EXCLUDE := G204,G304,G117,G402,G306,G302,G301,G703

.PHONY: build install test cover cover-html lint fmt sec vuln hooks tidy docs docs-check completions snapshot clean

# Point git at the tracked hooks in .githooks. Run once after cloning.
hooks:
	git config core.hooksPath .githooks
	@echo "git hooks installed (.githooks)"

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

# Install into ~/.local/bin. macOS kills a running binary overwritten in place, so remove first.
install: build
	mkdir -p $(HOME)/.local/bin
	rm -f $(HOME)/.local/bin/$(BIN)
	cp $(BIN) $(HOME)/.local/bin/$(BIN)

test:
	go test -race -shuffle=on ./...

# Tests with coverage and a no-regression floor (override MIN_COVER).
cover:
	sh scripts/coverage.sh

cover-html: cover
	go tool cover -html=coverage.out

fmt:
	gofmt -w .

lint:
	go vet ./...
	go run $(GOLANGCI) run ./...

sec:
	go run $(GOSEC) -quiet -exclude=$(GOSEC_EXCLUDE) ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

tidy:
	go mod tidy

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf $(BIN) dist/ coverage.out

# Regenerate man pages (man/) and per-command markdown (docs/cli/) from the cobra tree.
docs:
	go run ./tools/gendocs

# Verify docs generation is deterministic (used by CI): regenerate twice, diff.
docs-check:
	go run ./tools/gendocs
	cp -R docs/cli /tmp/eerox-docs-a && cp -R man /tmp/eerox-man-a
	go run ./tools/gendocs
	diff -r /tmp/eerox-docs-a docs/cli && diff -r /tmp/eerox-man-a man
	rm -rf /tmp/eerox-docs-a /tmp/eerox-man-a
	@echo "docs reproducible"

completions:
	sh scripts/completions.sh
