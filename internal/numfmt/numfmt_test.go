package numfmt

import "testing"

func TestTokens(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want string
	}{
		{"zero", 0, "0"},
		{"under_1k", 999, "999"},
		{"exactly_1k", 1_000, "1k"},
		{"between_1k_and_1M", 12_345, "12k"},
		{"just_under_1M", 999_999, "1000k"},
		{"exactly_1M", 1_000_000, "1M"},
		{"large_M", 23_500_000, "24M"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Tokens(tc.in)
			if got != tc.want {
				t.Errorf("Tokens(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
