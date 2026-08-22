package service

import (
	"archive/zip"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

const (
	metin2RootDataName          = "root.data"
	metin2LocaleCfgName         = "locale.cfg"
	metin2ServerIPPlaceholder   = "000.000.000.000"
	metin2LegacyServerIPLiteral = "185.171.90.185"
)

// LegacyMetin2ClientOptions contains the endpoint values embedded in the
// packed legacy client. The client authenticates on AuthPort and enters the
// first channel on WorldPort.
type LegacyMetin2ClientOptions struct {
	ServerIP  string
	AuthPort  int
	WorldPort int
}

// WriteLegacyMetin2ClientZip streams a per-server legacy Metin2 client. The
// base zip is copied without recompression except for root.data and
// locale.cfg. root.data uses a fixed-width endpoint placeholder because its
// companion index stores offsets into the data file.
func WriteLegacyMetin2ClientZip(base io.ReaderAt, size int64, out io.Writer, opts LegacyMetin2ClientOptions) error {
	reader, err := zip.NewReader(base, size)
	if err != nil {
		return err
	}
	if err := validateLegacyMetin2Options(opts); err != nil {
		return err
	}

	writer := zip.NewWriter(out)
	defer writer.Close()
	rootFound := false
	for _, file := range reader.File {
		switch {
		case strings.EqualFold(baseName(file.Name), metin2RootDataName):
			content, err := readZipFile(file)
			if err != nil {
				return err
			}
			patched, err := patchLegacyMetin2RootData(content, opts)
			if err != nil {
				return err
			}
			if err := writeZipEntry(writer, file.Name, patched); err != nil {
				return err
			}
			rootFound = true
		case strings.EqualFold(baseName(file.Name), metin2LocaleCfgName):
			if err := writeZipEntry(writer, file.Name, []byte("1252 pt")); err != nil {
				return err
			}
		default:
			if err := copyZipFile(writer, file); err != nil {
				return err
			}
		}
	}
	if !rootFound {
		return fmt.Errorf("legacy Metin2 client zip does not contain %s", metin2RootDataName)
	}
	return nil
}

// WriteLegacyMetin2ClientPatch writes only root.data and locale.cfg. The rest of
// the multi-gigabyte client stays on disk (or is served as the static base zip).
func WriteLegacyMetin2ClientPatch(base io.ReaderAt, size int64, out io.Writer, opts LegacyMetin2ClientOptions) error {
	reader, err := zip.NewReader(base, size)
	if err != nil {
		return err
	}
	if err := validateLegacyMetin2Options(opts); err != nil {
		return err
	}

	writer := zip.NewWriter(out)
	defer writer.Close()
	rootFound := false
	for _, file := range reader.File {
		switch {
		case strings.EqualFold(baseName(file.Name), metin2RootDataName):
			content, err := readZipFile(file)
			if err != nil {
				return err
			}
			patched, err := patchLegacyMetin2RootData(content, opts)
			if err != nil {
				return err
			}
			if err := writeZipEntry(writer, file.Name, patched); err != nil {
				return err
			}
			rootFound = true
		case strings.EqualFold(baseName(file.Name), metin2LocaleCfgName):
			if err := writeZipEntry(writer, file.Name, []byte("1252 pt")); err != nil {
				return err
			}
		}
	}
	if !rootFound {
		return fmt.Errorf("legacy Metin2 client zip does not contain %s", metin2RootDataName)
	}
	return nil
}

func validateLegacyMetin2Options(opts LegacyMetin2ClientOptions) error {
	if net.ParseIP(opts.ServerIP).To4() == nil {
		return fmt.Errorf("legacy Metin2 client requires a valid IPv4 server address")
	}
	for name, port := range map[string]int{"auth": opts.AuthPort, "world": opts.WorldPort} {
		if port < 1 || port > 65535 {
			return fmt.Errorf("legacy Metin2 %s port is invalid: %d", name, port)
		}
	}
	return nil
}

func patchLegacyMetin2RootData(content []byte, opts LegacyMetin2ClientOptions) ([]byte, error) {
	publicIP, err := formatLegacyMetin2IPv4(opts.ServerIP, len(metin2ServerIPPlaceholder))
	if err != nil {
		return nil, err
	}
	legacyPublicIP, err := formatLegacyMetin2IPv4(opts.ServerIP, len(metin2LegacyServerIPLiteral))
	if err != nil {
		return nil, err
	}
	privateIP, err := formatLegacyMetin2IPv4(opts.ServerIP, len("192.168.2.100"))
	if err != nil {
		return nil, err
	}
	patched := append([]byte(nil), content...)
	mainIPCount := replaceFixedBytes(patched, []byte(metin2LegacyServerIPLiteral), []byte(legacyPublicIP))
	mainIPCount += replaceFixedBytes(patched, []byte("127.000.00.001"), []byte(publicIP))
	mainIPCount += replaceFixedBytes(patched, []byte("192.168.2.100"), []byte(privateIP))
	mainIPCount += replaceFixedBytes(patched, []byte("127.000.000.1"), []byte(privateIP))
	mainIPCount += replaceFixedBytes(patched, []byte("127.000.000.001"), []byte(publicIP))
	mainIPCount += replaceFixedBytes(
		patched,
		[]byte(fmt.Sprintf(`SERVER_IP = "%s"`, metin2ServerIPPlaceholder)),
		[]byte(fmt.Sprintf(`SERVER_IP = "%s"`, publicIP)),
	)
	if mainIPCount == 0 {
		return nil, fmt.Errorf("legacy Metin2 root.data does not contain a known server endpoint")
	}

	worldPort := fmt.Sprintf("%05d", opts.WorldPort)
	authPort := fmt.Sprintf("%05d", opts.AuthPort)
	for _, old := range []string{"13001", "13101"} {
		replaceFixedBytes(patched, []byte("CH1_PORT = "+old), []byte("CH1_PORT = "+worldPort))
		replaceFixedBytes(patched, []byte("MARKADDR = "+old), []byte("MARKADDR = "+worldPort))
	}
	for _, old := range []string{"11000", "11100"} {
		replaceFixedBytes(patched, []byte("AUTH_PORT = "+old), []byte("AUTH_PORT = "+authPort))
	}
	return patched, nil
}

func formatLegacyMetin2IPv4(value string, targetLength int) (string, error) {
	ip := net.ParseIP(value).To4()
	if ip == nil || targetLength < 7 {
		return "", fmt.Errorf("legacy Metin2 client requires a valid IPv4 endpoint")
	}

	decimalParts := make([]string, len(ip))
	hexParts := make([]string, len(ip))
	for i, part := range ip {
		decimalParts[i] = strconv.Itoa(int(part))
		hexParts[i] = fmt.Sprintf("0x%X", part)
	}

	for hexCount := 0; hexCount <= len(ip); hexCount++ {
		for mask := 0; mask < 1<<len(ip); mask++ {
			if countBits(mask) != hexCount {
				continue
			}
			parts := append([]string(nil), decimalParts...)
			for i := range parts {
				if mask&(1<<i) != 0 {
					parts[i] = hexParts[i]
				}
			}
			candidate := strings.Join(parts, ".")
			if len(candidate) == targetLength {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("server IPv4 %q cannot fit in legacy Metin2 root.data", value)
}

func countBits(value int) int {
	count := 0
	for value != 0 {
		count += value & 1
		value >>= 1
	}
	return count
}

func replaceFixedBytes(content, oldValue, newValue []byte) int {
	if len(oldValue) != len(newValue) {
		return 0
	}
	count := 0
	for offset := 0; offset <= len(content)-len(oldValue); offset++ {
		if string(content[offset:offset+len(oldValue)]) != string(oldValue) {
			continue
		}
		copy(content[offset:offset+len(newValue)], newValue)
		count++
		offset += len(oldValue) - 1
	}
	return count
}

func writeZipEntry(writer *zip.Writer, name string, content []byte) error {
	target, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = target.Write(content)
	return err
}
