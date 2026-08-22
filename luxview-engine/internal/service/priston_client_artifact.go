package service

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
)

const (
	pristonRegName     = "ptreg.rgx"
	pristonLauncherINI = "openpriston.launcher.ini"
	pristonDefaultName = "LuxView"
	pristonDefaultPort = 10012
)

var (
	pristonQuotedEntry = regexp.MustCompile(`(?i)("Server(?:1|2|3|Name)")\s+"[^"]*"`)
	pristonIniLine     = regexp.MustCompile(`(?im)^(ServerAddress|ServerPort|ServerName)=.*$`)
)

type PristonClientOptions struct {
	ServerName string
	ServerIP   string
	GamePort   int
}

func WritePristonClientZip(base io.ReaderAt, size int64, out io.Writer, opts PristonClientOptions) error {
	if opts.ServerIP == "" {
		opts.ServerIP = "127.0.0.1"
	}
	if opts.GamePort == 0 {
		opts.GamePort = pristonDefaultPort
	}
	if strings.TrimSpace(opts.ServerName) == "" {
		opts.ServerName = pristonDefaultName
	}

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

// WritePristonClientPatch writes only ptreg.rgx and openpriston.launcher.ini.
func WritePristonClientPatch(base io.ReaderAt, size int64, out io.Writer, opts PristonClientOptions) error {
	if opts.ServerIP == "" {
		opts.ServerIP = "127.0.0.1"
	}
	if opts.GamePort == 0 {
		opts.GamePort = pristonDefaultPort
	}
	if strings.TrimSpace(opts.ServerName) == "" {
		opts.ServerName = pristonDefaultName
	}

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

func isPristonReg(name string) bool {
	return strings.EqualFold(baseName(name), pristonRegName)
}

func isPristonLauncherINI(name string) bool {
	return strings.EqualFold(baseName(name), pristonLauncherINI)
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

func buildPristonLauncherINI(opts PristonClientOptions) string {
	return "# Perfil do launcher LuxView.\n" +
		"ServerAddress=" + opts.ServerIP + "\n" +
		"ServerPort=" + itoa(opts.GamePort) + "\n" +
		"ServerName=" + opts.ServerName + "\n" +
		"GameExecutable=game.exe\n"
}
