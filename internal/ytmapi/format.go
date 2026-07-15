package ytmapi

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// FormatCount formats view / play counts for compact display.
// Accepts raw integers ("1234567"), comma-grouped ("1,234,567"),
// or already-abbreviated values ("12M", "1.2K views").
func FormatCount(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, ",", "")

	lower := strings.ToLower(s)
	for _, suf := range []string{
		" views", " view", " plays", " play", " watching", " listeners",
	} {
		if strings.HasSuffix(lower, suf) {
			s = strings.TrimSpace(s[:len(s)-len(suf)])
			lower = strings.ToLower(s)
			break
		}
	}
	if s == "" {
		return ""
	}

	// Already abbreviated (e.g. 12M, 1.2k).
	if n := len(s); n >= 2 {
		last := rune(s[n-1])
		if last == 'k' || last == 'K' || last == 'm' || last == 'M' || last == 'b' || last == 'B' {
			num := s[:n-1]
			if _, err := strconv.ParseFloat(num, 64); err == nil {
				return trimDotZero(num) + string(unicode.ToUpper(last))
			}
		}
	}

	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return compactInt(n)
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return compactInt(int64(f + 0.5))
	}
	return strings.TrimSpace(raw)
}

// FormatCountAny formats counts that may arrive as JSON numbers (float64/int) or strings.
func FormatCountAny(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return FormatCount(x)
	case float64:
		return compactInt(int64(x + 0.5))
	case float32:
		return compactInt(int64(x + 0.5))
	case int:
		return compactInt(int64(x))
	case int64:
		return compactInt(x)
	case int32:
		return compactInt(int64(x))
	default:
		return FormatCount(fmt.Sprint(x))
	}
}

func compactInt(n int64) string {
	if n < 0 {
		n = -n
	}
	switch {
	case n >= 1_000_000_000:
		return trimDotZero(fmt.Sprintf("%.1f", float64(n)/1_000_000_000)) + "B"
	case n >= 1_000_000:
		return trimDotZero(fmt.Sprintf("%.1f", float64(n)/1_000_000)) + "M"
	case n >= 1_000:
		return trimDotZero(fmt.Sprintf("%.1f", float64(n)/1_000)) + "K"
	default:
		return strconv.FormatInt(n, 10)
	}
}

func trimDotZero(s string) string {
	if strings.HasSuffix(s, ".0") {
		return s[:len(s)-2]
	}
	return s
}
