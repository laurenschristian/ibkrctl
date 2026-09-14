// Command gendocs regenerates man pages (man/) and per-command markdown (docs/cli/)
// from the live cobra command tree. Output is gitignored; run `make docs` for an
// offline reference. Reproducibility is checked by `make docs-check`.
package main

import (
	"log"
	"os"
	"time"

	"github.com/spf13/cobra/doc"

	"github.com/laurenschristian/ibkrctl/internal/cli"
)

func main() {
	root := cli.Root()
	root.DisableAutoGenTag = true
	if err := os.MkdirAll("man", 0o750); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll("docs/cli", 0o750); err != nil {
		log.Fatal(err)
	}
	hdr := &doc.GenManHeader{Title: "IBKRCTL", Section: "1", Date: &time.Time{}, Source: "ibkrctl", Manual: "ibkrctl manual"}
	if err := doc.GenManTree(root, hdr, "man"); err != nil {
		log.Fatal(err)
	}
	if err := doc.GenMarkdownTree(root, "docs/cli"); err != nil {
		log.Fatal(err)
	}
}
