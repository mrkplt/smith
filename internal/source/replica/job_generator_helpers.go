package replica

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"strings"
)

func sanitizeEtcdEndpoints(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, raw := range in {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func skillConfigMapName(source string) string {
	trimmed := strings.TrimSpace(source)
	name := strings.TrimPrefix(trimmed, "local://skills/")
	name = sanitizeName(name)
	if name == "" {
		name = "unknown"
	}
	return "skill-" + name
}

func sanitizeName(loopID string) string {
	s := strings.ToLower(strings.TrimSpace(loopID))
	replacer := strings.NewReplacer(
		"/", "-",
		"_", "-",
		".", "-",
		" ", "-",
	)
	s = replacer.Replace(s)
	s = strings.Trim(s, "-")
	if s == "" {
		return "loop"
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

func sanitizeKubernetesLabelValue(raw string) string {
	const maxLen = 63
	s := strings.TrimSpace(raw)
	if s == "" {
		return "unknown"
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}

	out := strings.Trim(b.String(), "-_.")
	if out == "" {
		out = "unknown"
	}
	if len(out) <= maxLen {
		return out
	}
	sum := sha256.Sum256([]byte(out))
	suffix := hex.EncodeToString(sum[:])[:8]
	keep := maxLen - 1 - len(suffix)
	if keep < 1 {
		keep = 1
	}
	out = strings.Trim(out[:keep], "-_.")
	if out == "" {
		out = "x"
	}
	return out + "-" + suffix
}

func workspacePathUnsafe(raw string) bool {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "/") {
		return true
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." {
		return true
	}
	if strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return true
	}
	return false
}

func workspacePRDAbsolutePath(raw string) string {
	relative := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if relative == "" || workspacePathUnsafe(relative) {
		relative = ".agents/tasks/prd.json"
	}
	cleaned := path.Clean(relative)
	cleaned = strings.TrimPrefix(cleaned, "./")
	return path.Join("/workspace", cleaned)
}

func shellQuote(raw string) string {
	escaped := strings.ReplaceAll(raw, `'`, `'\''`)
	return "'" + escaped + "'"
}
