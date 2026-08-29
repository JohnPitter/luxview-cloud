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

// muLogin mirrors the engine's MuLogin (PristonLogin clipped to 10 chars):
// the MU client login field and the OpenMU account name accept at most 10.
func muLogin(username string) string {
	var b strings.Builder
	for _, r := range username {
		if r > unicode.MaxASCII {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		}
	}
	login := clipRunes(b.String(), 10)
	if login == "" {
		return ""
	}
	if login[0] < 'A' || (login[0] > 'Z' && login[0] < 'a') || login[0] > 'z' {
		login = clipRunes("p"+login, 10)
	}
	return login
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
