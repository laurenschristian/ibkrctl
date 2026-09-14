package cli

import (
	"encoding/json"
	"strings"
)

// sensitiveKeys are blanked in redacted output (personal names / titles).
var sensitiveKeys = map[string]bool{
	"accountTitle": true,
	"displayName":  true,
	"accountAlias": true,
	"acctCustType": true,
}

// redact returns v with real account ids replaced by their aliases and personal
// name fields blanked, when Redact is enabled. Typed values are normalized to
// generic JSON first so the whole tree is walked.
func redact(v any) any {
	if cfg == nil || !cfg.Redact {
		return v
	}
	m := cfg.RedactMap()
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var g any
	if err := json.Unmarshal(b, &g); err != nil {
		return v
	}
	return walk(g, m)
}

func walk(v any, m map[string]string) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			nk := replaceIDs(k, m)
			if sensitiveKeys[k] {
				out[nk] = "***"
				continue
			}
			out[nk] = walk(val, m)
		}
		return out
	case []any:
		for i := range t {
			t[i] = walk(t[i], m)
		}
		return t
	case string:
		return replaceIDs(t, m)
	default:
		return v
	}
}

func replaceIDs(s string, m map[string]string) string {
	for id, alias := range m {
		if strings.Contains(s, id) {
			s = strings.ReplaceAll(s, id, alias)
		}
	}
	return s
}
