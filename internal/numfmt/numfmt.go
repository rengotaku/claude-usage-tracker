// Package numfmt provides numeric formatting helpers shared across CLI/report output.
package numfmt

import "fmt"

// Tokens formats an integer token count using k/M suffixes.
//
//	n >= 1_000_000 -> "<x>M"
//	n >= 1_000     -> "<x>k"
//	otherwise      -> raw integer
func Tokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.0fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.0fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
