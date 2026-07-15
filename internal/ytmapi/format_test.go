package ytmapi

import "testing"

func TestFormatCount(t *testing.T) {
	cases := map[string]string{
		"":              "",
		"42":            "42",
		"999":           "999",
		"1000":          "1K",
		"1500":          "1.5K",
		"1234567":       "1.2M",
		"1,234,567":     "1.2M",
		"12M":           "12M",
		"1.2k views":    "1.2K",
		"9800000000":    "9.8B",
		"12M views":     "12M",
		"3.4M plays":    "3.4M",
	}
	for in, want := range cases {
		if got := FormatCount(in); got != want {
			t.Errorf("FormatCount(%q)=%q want %q", in, got, want)
		}
	}
}
