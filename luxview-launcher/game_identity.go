package main

import (
	"strings"
	"unicode"
)

func tibiaEmail(username string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "@luxviewot.com"
}

func rakionLogin(username string) string {
	var b strings.Builder
	for _, r := range username {
		if r > unicode.MaxASCII {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return clipRunes(b.String(), 11)
}

func rakionPassword(password string) string {
	return clipRunes(strings.ToLower(password), 11)
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
