package api

import "strconv"

// Escapes with Go's own rules, which cover the control characters a terminal
// acts on and the bidirectional overrides that reorder what is around them.
func Printable(text string) string {
	quoted := strconv.QuoteToGraphic(text)

	return quoted[1 : len(quoted)-1]
}
