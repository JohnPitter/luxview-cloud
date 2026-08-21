package service

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

const tibiaMailDomain = "luxviewot.com"

func TibiaEmail(username string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "@" + tibiaMailDomain
}

// TibiaCharacterName is a default in-game name derived from the LuxView username.
func TibiaCharacterName(username string) string {
	name, err := ParseTibiaCharacterName(username)
	if err != nil {
		return ""
	}
	return name
}

// ParseTibiaCharacterName keeps letters and spaces, title-cases words, 2–29 chars.
func ParseTibiaCharacterName(name string) (string, error) {
	var words []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		w := cur.String()
		cur.Reset()
		words = append(words, strings.ToUpper(w[:1])+strings.ToLower(w[1:]))
	}
	for _, r := range strings.TrimSpace(name) {
		if r > unicode.MaxASCII {
			continue
		}
		if unicode.IsLetter(r) {
			cur.WriteRune(r)
			continue
		}
		if r == ' ' {
			flush()
		}
	}
	flush()
	out := strings.Join(words, " ")
	if len(out) < 2 || len(out) > 29 {
		return "", fmt.Errorf("nome do personagem: 2 a 29 letras")
	}
	return out, nil
}

// TibiaVocationSample is the Canary template character copied for a new player.
func TibiaVocationSample(vocation string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(vocation)) {
	case "knight", "cavaleiro":
		return "Knight Sample", nil
	case "paladin", "paladino":
		return "Paladin Sample", nil
	case "sorcerer", "mago":
		return "Sorcerer Sample", nil
	case "druid", "druida":
		return "Druid Sample", nil
	case "monk", "monge":
		return "Monk Sample", nil
	default:
		return "", fmt.Errorf("escolha a vocação")
	}
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
