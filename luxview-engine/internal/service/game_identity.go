package service

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"unicode"
)

const tibiaMailDomain = "luxviewot.com"

func TibiaEmail(username string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "@" + tibiaMailDomain
}

func MetinLogin(username string) string {
	return clipRunes(strings.TrimSpace(username), 30)
}

func RakionLogin(username string) string {
	var b strings.Builder
	for _, r := range username {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if r > unicode.MaxASCII {
				continue
			}
			b.WriteRune(r)
		}
	}
	return clipRunes(b.String(), 11)
}

func RakionPassword(password string) string {
	return clipRunes(strings.ToLower(password), 11)
}

func SHA1Hex(plain string) string {
	sum := sha1.Sum([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// MySQLPassword is MariaDB/MySQL PASSWORD() (4.1+): '*' + SHA1(SHA1(plain)) uppercase hex.
func MySQLPassword(plain string) string {
	h1 := sha1.Sum([]byte(plain))
	h2 := sha1.Sum(h1[:])
	return "*" + strings.ToUpper(hex.EncodeToString(h2[:]))
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func mysqlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "'", `\'`)
	s = strings.ReplaceAll(s, "\x00", "")
	return "'" + s + "'"
}
