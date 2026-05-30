package service

import (
	"archive/zip"
	"crypto/aes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"strings"
)

// Rakion client config.xfs generation (Go port of tools/gconfig.py).
//
// config.xfs are 2 Base64 lines separated by CRLF:
//
//	line1 ([server]) = AES-ECB( "[server]\nip@<HOST>\n",            randomKey )
//	line2 ([sec])    = AES-ECB( "[sec]\nkey@<b64(rk)>\niv@<b64(riv)>\n", FIXED_KEY )
//
// The client decrypts line2 with the fixed key to recover the random key, then
// decrypts line1 with it to obtain the auth-web host. ECB mode (the IV is only
// carried in the format, not used by the cipher). Plaintext is space-padded to
// a multiple of 16 (no PKCS7).
const (
	rakionFixedKey   = "s&a3edecuwuy@ye*"                   // 16-byte AES key from GConfig
	rakionCharset    = "bcdefghijkmnopqrstuvwxyz023456789*" // alphabet for the random key/iv
	rakionConfigName = "config.xfs"
)

// aesECBSpacePadB64 encrypts text with AES-ECB (space-padded to a 16-byte
// multiple) and returns the standard Base64 of the ciphertext.
func aesECBSpacePadB64(text string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	data := []byte(text)
	if pad := len(data) % aes.BlockSize; pad != 0 {
		data = append(data, spaces(aes.BlockSize-pad)...)
	}
	ct := make([]byte, len(data))
	for i := 0; i < len(data); i += aes.BlockSize {
		block.Encrypt(ct[i:i+aes.BlockSize], data[i:i+aes.BlockSize])
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

func spaces(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return b
}

// rakionRandToken returns n random characters drawn from rakionCharset.
func rakionRandToken(n int) (string, error) {
	var sb strings.Builder
	max := big.NewInt(int64(len(rakionCharset)))
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		sb.WriteByte(rakionCharset[idx.Int64()])
	}
	return sb.String(), nil
}

// BuildRakionConfigXfs builds the config.xfs payload pointing the client at the
// given auth-web host (IP or hostname, e.g. "rakion.luxview.cloud").
func BuildRakionConfigXfs(host string) ([]byte, error) {
	rk, err := rakionRandToken(16)
	if err != nil {
		return nil, err
	}
	riv, err := rakionRandToken(16)
	if err != nil {
		return nil, err
	}

	line1, err := aesECBSpacePadB64(fmt.Sprintf("[server]\nip@%s\n", host), []byte(rk))
	if err != nil {
		return nil, err
	}
	sec := fmt.Sprintf("[sec]\nkey@%s\niv@%s\n",
		base64.StdEncoding.EncodeToString([]byte(rk)),
		base64.StdEncoding.EncodeToString([]byte(riv)))
	line2, err := aesECBSpacePadB64(sec, []byte(rakionFixedKey))
	if err != nil {
		return nil, err
	}
	return []byte(line1 + "\r\n" + line2 + "\r\n"), nil
}

// RakionClientOptions configures a per-server Rakion client download.
type RakionClientOptions struct {
	AuthHost string // host the client's config.xfs points at (auth web)
}

// WriteRakionClientZip streams the base client zip to out, replacing every
// config.xfs entry (e.g. Bin/config.xfs and the root config.xfs) with one
// freshly generated for opts.AuthHost. All other entries are copied raw (no
// recompression) so a large client stays fast.
func WriteRakionClientZip(base io.ReaderAt, size int64, out io.Writer, opts RakionClientOptions) error {
	reader, err := zip.NewReader(base, size)
	if err != nil {
		return err
	}

	configXfs, err := BuildRakionConfigXfs(opts.AuthHost)
	if err != nil {
		return err
	}

	writer := zip.NewWriter(out)
	defer writer.Close()

	for _, file := range reader.File {
		if isRakionConfigEntry(file.Name) {
			w, err := writer.Create(file.Name)
			if err != nil {
				return err
			}
			if _, err := w.Write(configXfs); err != nil {
				return err
			}
			continue
		}
		if err := copyZipFile(writer, file); err != nil {
			return err
		}
	}
	return nil
}

// isRakionConfigEntry reports whether a zip entry's base name is config.xfs
// (matches both "config.xfs" and "Bin/config.xfs", case-insensitively).
func isRakionConfigEntry(name string) bool {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return strings.EqualFold(name, rakionConfigName)
}
