#!/bin/sh
# Run the test suite with coverage and enforce a no-regression floor. Ratchet
# MIN_COVER up over time; override with MIN_COVER=nn. Used by make cover, the
# pre-push hook and CI.
set -e

MIN_COVER=${MIN_COVER:-50}  # browser (chromedp) + launchd + system discovery are integration-only
if ! go test -race -shuffle=on -coverprofile=coverage.out ./...; then
	echo "coverage: tests failed while collecting profile" >&2
	exit 1
fi
total=$(go tool cover -func=coverage.out | awk '/^total:/{gsub(/%/,"",$3); print $3}')
echo "total coverage: ${total}% (floor ${MIN_COVER}%)"
below=$(awk -v t="$total" -v m="$MIN_COVER" 'BEGIN{print (t+0 < m+0) ? "1" : "0"}')
if [ "$below" = "1" ]; then
	echo "coverage ${total}% is below the floor ${MIN_COVER}%" >&2
	exit 1
fi
