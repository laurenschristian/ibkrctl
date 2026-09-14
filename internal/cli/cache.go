package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/laurenschristian/ibkrctl/internal/config"
)

// historyTTL is short for intraday bars (they move) and long for daily+ bars.
func historyTTL(bar string) time.Duration {
	b := strings.ToLower(bar)
	if strings.HasSuffix(b, "d") || strings.HasSuffix(b, "w") || strings.HasSuffix(b, "m") {
		return 12 * time.Hour
	}
	return 10 * time.Minute
}

func historyCacheKey(conid, period, bar string, outside bool) string {
	sum := sha256.Sum256([]byte(conid + "|" + period + "|" + bar + "|" + boolStr(outside)))
	return hex.EncodeToString(sum[:16])
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// cachedHistory serves bars from disk when fresh, else fetches and caches them.
// A cache failure never blocks the request; it just falls through to the API.
func cachedHistory(ctx context.Context, conid, period, bar string, outside, refresh bool) (any, error) {
	dir := filepath.Join(config.CacheDir(), "history")
	path := filepath.Join(dir, historyCacheKey(conid, period, bar, outside)+".json")
	if !refresh {
		if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) < historyTTL(bar) {
			if b, err := os.ReadFile(path); err == nil {
				var v any
				if json.Unmarshal(b, &v) == nil {
					return v, nil
				}
			}
		}
	}
	data, err := client.History(ctx, conid, period, bar, outside)
	if err != nil {
		return nil, err
	}
	if b, err := json.Marshal(data); err == nil {
		if os.MkdirAll(dir, 0o700) == nil {
			_ = os.WriteFile(path, b, 0o600)
		}
	}
	return data, nil
}
