package common

import (
	"strings"
	"unicode"
)

func NormalizeText(s string) string {
	if s == "\u00a0" {
		return s
	}
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == '\u00a0' {
			if space {
				b.WriteRune(' ')
				space = false
			}
			b.WriteRune(r)
			continue
		}
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteRune(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

func TrimInlineText(inlines []irInline) []irInline {
	return inlines
}

type irInline interface{}
