package utils

import (
	"strings"
	"unicode"
)

func StringsAllowlist(s string, valid []*unicode.RangeTable) string {
	return strings.Map(
		func(r rune) rune {
			if unicode.IsOneOf(valid, r) {
				return r
			}
			return -1
		},
		s,
	)
}
