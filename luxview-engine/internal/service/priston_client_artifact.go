package service

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
)

const (
	pristonRegName         = "ptreg.rgx"
	pristonLauncherINI     = "openpriston.launcher.ini"
	pristonGameINI         = "game.ini"
	pristonGameExe         = "game.exe"
	pristonDefaultName     = "LuxView"
	pristonDefaultPort     = 10012
	pristonDefaultClanPort = 10013
)

var (
	pristonQuotedEntry = regexp.MustCompile(`(?i)("Server(?:1|2|3|Name)")\s+"[^"]*"`)
	pristonIniLine     = regexp.MustCompile(`(?im)^(ServerAddress|ServerPort|ServerName)=.*$`)
	pristonGameIniLine = regexp.MustCompile(`(?im)^(IP|Port|Clan)\s*=.*$`)
	pristonBPTConnect  = []byte("189.46.228.170:30303")
)

type PristonClientOptions struct {
	ServerName string
	ServerIP   string
	GamePort   int
	ClanPort   int
}

func WritePristonClientZip(base io.ReaderAt, size int64, out io.Writer, opts PristonClientOptions) error {
	normalizePristonClientOptions(&opts)

	reader, err := zip.NewReader(base, size)
	if err != nil {
		return err
	}

	writer := zip.NewWriter(out)
	defer writer.Close()

	wroteINI := false
	for _, file := range reader.File {
		switch {
		case isPristonReg(file.Name):
			if err := writePatchedPristonReg(writer, file, opts); err != nil {
				return err
			}
		case isPristonLauncherINI(file.Name):
			if err := writePatchedPristonINI(writer, file, opts); err != nil {
				return err
			}
			wroteINI = true
		case isPristonGameINI(file.Name):
			if err := writePatchedPristonGameINI(writer, file, opts); err != nil {
				return err
			}
		case isPristonGameExe(file.Name):
			if err := writePatchedPristonGameExe(writer, file, opts); err != nil {
				return err
			}
		default:
			if err := copyZipFile(writer, file); err != nil {
				return err
			}
		}
	}
	if wroteINI {
		return nil
	}
	header, err := writer.Create(pristonLauncherINI)
	if err != nil {
		return err
	}
	_, err = io.WriteString(header, buildPristonLauncherINI(opts))
	return err
}

// WritePristonClientPatch writes ptreg.rgx, launcher.ini, game.ini and Game.exe
// (the Reloaded client reads ConnectServer from game.ini / a BPT IP inside the exe).
func WritePristonClientPatch(base io.ReaderAt, size int64, out io.Writer, opts PristonClientOptions) error {
	normalizePristonClientOptions(&opts)

	reader, err := zip.NewReader(base, size)
	if err != nil {
		return err
	}

	writer := zip.NewWriter(out)
	defer writer.Close()

	wroteINI := false
	for _, file := range reader.File {
		switch {
		case isPristonReg(file.Name):
			if err := writePatchedPristonReg(writer, file, opts); err != nil {
				return err
			}
		case isPristonLauncherINI(file.Name):
			if err := writePatchedPristonINI(writer, file, opts); err != nil {
				return err
			}
			wroteINI = true
		case isPristonGameINI(file.Name):
			if err := writePatchedPristonGameINI(writer, file, opts); err != nil {
				return err
			}
		case isPristonGameExe(file.Name):
			if err := writePatchedPristonGameExe(writer, file, opts); err != nil {
				return err
			}
		}
	}
	if wroteINI {
		return nil
	}
	header, err := writer.Create(pristonLauncherINI)
	if err != nil {
		return err
	}
	_, err = io.WriteString(header, buildPristonLauncherINI(opts))
	return err
}

func normalizePristonClientOptions(opts *PristonClientOptions) {
	if opts.ServerIP == "" {
		opts.ServerIP = "127.0.0.1"
	}
	if opts.GamePort == 0 {
		opts.GamePort = pristonDefaultPort
	}
	if opts.ClanPort == 0 {
		opts.ClanPort = pristonDefaultClanPort
	}
	if strings.TrimSpace(opts.ServerName) == "" {
		opts.ServerName = pristonDefaultName
	}
}

func isPristonReg(name string) bool {
	return strings.EqualFold(baseName(name), pristonRegName)
}

func isPristonLauncherINI(name string) bool {
	return strings.EqualFold(baseName(name), pristonLauncherINI)
}

func isPristonGameINI(name string) bool {
	return strings.EqualFold(baseName(name), pristonGameINI)
}

func isPristonGameExe(name string) bool {
	return strings.EqualFold(baseName(name), pristonGameExe)
}

func writePatchedPristonReg(writer *zip.Writer, file *zip.File, opts PristonClientOptions) error {
	content, err := readZipFile(file)
	if err != nil {
		return err
	}
	return writeReplacedZipFile(writer, file, patchPristonReg(content, opts))
}

func writePatchedPristonINI(writer *zip.Writer, file *zip.File, opts PristonClientOptions) error {
	content, err := readZipFile(file)
	if err != nil {
		return err
	}
	patched := patchPristonINI(content, opts)
	if len(patched) == 0 {
		patched = []byte(buildPristonLauncherINI(opts))
	}
	return writeReplacedZipFile(writer, file, patched)
}

func writePatchedPristonGameINI(writer *zip.Writer, file *zip.File, opts PristonClientOptions) error {
	content, err := readZipFile(file)
	if err != nil {
		return err
	}
	return writeReplacedZipFile(writer, file, patchPristonGameINI(content, opts))
}

func writePatchedPristonGameExe(writer *zip.Writer, file *zip.File, opts PristonClientOptions) error {
	content, err := readZipFile(file)
	if err != nil {
		return err
	}
	patched := patchPristonGameExe(content, opts)
	if bytes.Equal(patched, content) {
		return copyZipFile(writer, file)
	}
	return writeReplacedZipFile(writer, file, patched)
}

func writeReplacedZipFile(writer *zip.Writer, file *zip.File, content []byte) error {
	header := file.FileHeader
	header.Method = zip.Deflate
	header.SetModTime(file.ModTime())
	target, err := writer.CreateHeader(&header)
	if err != nil {
		return err
	}
	_, err = target.Write(content)
	return err
}

func patchPristonReg(content []byte, opts PristonClientOptions) []byte {
	replaced := pristonQuotedEntry.ReplaceAllFunc(content, func(match []byte) []byte {
		key := pristonQuotedEntry.FindSubmatch(match)[1]
		value := opts.ServerIP
		if bytes.EqualFold(key, []byte(`"ServerName"`)) {
			value = opts.ServerName
		}
		var b strings.Builder
		b.Write(key)
		b.WriteString(` "`)
		b.WriteString(value)
		b.WriteByte('"')
		return []byte(b.String())
	})
	return replaced
}

func patchPristonINI(content []byte, opts PristonClientOptions) []byte {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil
	}
	values := map[string]string{
		"ServerAddress": opts.ServerIP,
		"ServerPort":    itoa(opts.GamePort),
		"ServerName":    opts.ServerName,
	}
	return pristonIniLine.ReplaceAllFunc(content, func(match []byte) []byte {
		parts := bytes.SplitN(match, []byte("="), 2)
		if len(parts) == 0 {
			return match
		}
		key := string(bytes.TrimSpace(parts[0]))
		if value, ok := values[key]; ok {
			return []byte(key + "=" + value)
		}
		return match
	})
}

func patchPristonGameINI(content []byte, opts PristonClientOptions) []byte {
	if len(bytes.TrimSpace(content)) == 0 {
		return []byte(buildPristonGameINI(opts))
	}
	values := map[string]string{
		"IP":   opts.ServerIP,
		"Port": itoa(opts.GamePort),
		"Clan": opts.ServerIP + ":" + itoa(opts.ClanPort),
	}
	return pristonGameIniLine.ReplaceAllFunc(content, func(match []byte) []byte {
		parts := bytes.SplitN(match, []byte("="), 2)
		if len(parts) == 0 {
			return match
		}
		key := string(bytes.TrimSpace(parts[0]))
		value, ok := values[key]
		if !ok {
			return match
		}
		return []byte(key + "=" + value)
	})
}

func patchPristonGameExe(content []byte, opts PristonClientOptions) []byte {
	replacement := opts.ServerIP + ":" + itoa(opts.GamePort)
	if len(replacement) > len(pristonBPTConnect) || !bytes.Contains(content, pristonBPTConnect) {
		return content
	}
	padded := make([]byte, len(pristonBPTConnect))
	copy(padded, replacement)
	return bytes.ReplaceAll(content, pristonBPTConnect, padded)
}

func buildPristonLauncherINI(opts PristonClientOptions) string {
	return "# Perfil do launcher LuxView.\n" +
		"ServerAddress=" + opts.ServerIP + "\n" +
		"ServerPort=" + itoa(opts.GamePort) + "\n" +
		"ServerName=" + opts.ServerName + "\n" +
		"GameExecutable=game.exe\n"
}

func buildPristonGameINI(opts PristonClientOptions) string {
	return "[ConnectServer]\r\nIP=" + opts.ServerIP + "\r\nPort=" + itoa(opts.GamePort) +
		"\r\nClan=" + opts.ServerIP + ":" + itoa(opts.ClanPort) + "\r\n"
}
