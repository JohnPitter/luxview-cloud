package service

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const muDefaultChatPort = 55980

const (
	openMULauncherConfigName = "launcher.config"
	openMUMainExePath        = "main.exe"
)

type OpenMUClientOptions struct {
	ServerName string
	ServerIP   string
	GamePort   int
}

type openMULauncherSettings struct {
	XMLName     xml.Name               `xml:"LauncherSettings"`
	MainExePath string                 `xml:"MainExePath"`
	Hosts       []openMUServerSettings `xml:"Hosts>ServerHostSettings"`
}

type openMUServerSettings struct {
	Description string `xml:"Description"`
	Address     string `xml:"Address"`
	Port        int    `xml:"Port"`
}

func BuildOpenMULauncherConfig(serverName string, serverIP string, gamePort int) string {
	settings := openMULauncherSettings{
		MainExePath: openMUMainExePath,
		Hosts: []openMUServerSettings{
			{
				Description: serverName,
				Address:     serverIP,
				Port:        gamePort,
			},
		},
	}
	data, err := xml.MarshalIndent(settings, "", "  ")
	if err != nil {
		return ""
	}
	return xml.Header + string(data) + "\n"
}

func WriteOpenMUClientZip(base io.ReaderAt, size int64, out io.Writer, opts OpenMUClientOptions) error {
	reader, err := zip.NewReader(base, size)
	if err != nil {
		return err
	}

	writer := zip.NewWriter(out)
	defer writer.Close()

	for _, file := range reader.File {
		if strings.EqualFold(file.Name, openMULauncherConfigName) {
			continue
		}
		if isMuServerInfoPath(file.Name) {
			if err := writePatchedMuServerInfo(writer, file, opts); err != nil {
				return err
			}
			continue
		}
		if isMuPackedIPPath(file.Name) {
			if err := writePatchedMuPackedIP(writer, file, opts); err != nil {
				return err
			}
			continue
		}
		if err := copyZipFile(writer, file); err != nil {
			return err
		}
	}

	configWriter, err := writer.Create(openMULauncherConfigName)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(configWriter, BuildOpenMULauncherConfig(opts.ServerName, opts.ServerIP, opts.GamePort)); err != nil {
		return err
	}
	return writeMuEncTerrainStubs(writer, reader.File)
}

// WriteOpenMUClientPatch writes launcher.config, Main.exe, the Native AOT
// client library, flags, EncTerrain*.att, and EncTerrain.obj stubs for Season 9
// worlds that shipped without object files.
func WriteOpenMUClientPatch(base io.ReaderAt, size int64, out io.Writer, opts OpenMUClientOptions) error {
	writer := zip.NewWriter(out)
	defer writer.Close()
	configWriter, err := writer.Create(openMULauncherConfigName)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(configWriter, BuildOpenMULauncherConfig(opts.ServerName, opts.ServerIP, opts.GamePort)); err != nil {
		return err
	}
	if base == nil || size == 0 {
		return nil
	}
	reader, err := zip.NewReader(base, size)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		switch {
		case isMuServerInfoPath(file.Name):
			if err := writePatchedMuServerInfo(writer, file, opts); err != nil {
				return err
			}
		case isMuPackedIPPath(file.Name):
			if err := writePatchedMuPackedIP(writer, file, opts); err != nil {
				return err
			}
		case isMuFlagTexturePath(file.Name), isMuWorldTerrainPatchPath(file.Name), isMuClientLibraryPath(file.Name):
			if err := copyZipFile(writer, file); err != nil {
				return err
			}
		}
	}
	return writeMuEncTerrainStubs(writer, reader.File)
}

var (
	muServerInfoIP       = regexp.MustCompile(`(?i)IP="[^"]*"`)
	muServerInfoPort     = regexp.MustCompile(`(?im)^Port=\d+`)
	muServerInfoChatPort = regexp.MustCompile(`(?im)^ChatPort=\d+`)
)

func isMuServerInfoPath(name string) bool {
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	return strings.EqualFold(base, "ServerInfo.bmd")
}

func isMuPackedIPPath(name string) bool {
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	return strings.EqualFold(base, "main.exe") || strings.EqualFold(base, "IGC.dll")
}

// isMuClientLibraryPath inclui a biblioteca Native AOT que o Main.exe carrega
// via LoadLibrary na pasta do client (MUnique.Client.Library.dll). Sem ela no
// patch incremental, o ATUALIZAR entrega um Main.exe novo que não conecta.
func isMuClientLibraryPath(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
	if strings.Contains(normalized, "/") {
		return false
	}
	base := path.Base(normalized)
	switch {
	case base == "munique.client.library.dll",
		base == "munique.client.library.runtimeconfig.json",
		base == "hostfxr.dll":
		return true
	case strings.HasPrefix(base, "munique.client.") && strings.HasSuffix(base, ".dll"):
		return true
	default:
		return false
	}
}

func isMuFlagTexturePath(name string) bool {
	switch strings.ToLower(strings.ReplaceAll(name, "\\", "/")) {
	case "data/object31/flag.ozj", "data/object31/flag.ozt", "data/object31/bkflag.ozj":
		return true
	default:
		return false
	}
}

// isMuWorldTerrainPatchPath includes EncTerrain*.att (plaza/map collision) in the
// per-server patch so ATUALIZAR delivers terrain edits without re-downloading 700MiB.
func isMuWorldTerrainPatchPath(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
	if !strings.Contains(normalized, "/world") {
		return false
	}
	base := path.Base(normalized)
	if !strings.HasPrefix(base, "encterrain") {
		return false
	}
	return strings.EqualFold(path.Ext(base), ".att")
}

var muPackedDefaultIP = []byte("192.168.0.168")

func patchMuPackedIP(raw []byte, ip string) []byte {
	repl := []byte(ip)
	if len(repl) == 0 || len(repl) > len(muPackedDefaultIP) || !bytes.Contains(raw, muPackedDefaultIP) {
		return raw
	}
	padded := make([]byte, len(muPackedDefaultIP))
	copy(padded, repl)
	return bytes.ReplaceAll(raw, muPackedDefaultIP, padded)
}

func writePatchedMuPackedIP(writer *zip.Writer, file *zip.File, opts OpenMUClientOptions) error {
	raw, err := readZipFile(file)
	if err != nil {
		return err
	}
	target, err := writer.Create(file.Name)
	if err != nil {
		return err
	}
	_, err = target.Write(patchMuPackedIP(raw, opts.ServerIP))
	return err
}

func patchMuServerInfo(raw []byte, ip string, port, chatPort int) []byte {
	if strings.TrimSpace(ip) == "" {
		return raw
	}
	out := muServerInfoIP.ReplaceAll(raw, []byte(`IP="`+ip+`"`))
	out = muServerInfoChatPort.ReplaceAll(out, []byte("ChatPort="+strconv.Itoa(chatPort)))
	out = muServerInfoPort.ReplaceAll(out, []byte("Port="+strconv.Itoa(port)))
	return out
}

func writePatchedMuServerInfo(writer *zip.Writer, file *zip.File, opts OpenMUClientOptions) error {
	raw, err := readZipFile(file)
	if err != nil {
		return err
	}
	target, err := writer.Create(file.Name)
	if err != nil {
		return err
	}
	_, err = target.Write(patchMuServerInfo(raw, opts.ServerIP, opts.GamePort, muDefaultChatPort))
	return err
}

var (
	muWorldPath     = regexp.MustCompile(`(?i)(?:^|/)world(\d+)(?:/|$)`)
	muEncTerrainRel = regexp.MustCompile(`(?i)^encterrain(\d+)\.(obj|map)$`)
	muMapXorTab     = [...]byte{
		0xd1, 0x73, 0x52, 0xf6, 0xd2, 0x9a, 0xcb, 0x27,
		0x3e, 0xaf, 0x59, 0x31, 0x37, 0xb3, 0xe7, 0xa2,
	}
)

func muDecryptMapFile(in []byte) []byte {
	out := make([]byte, len(in))
	key := byte(0x5e)
	for i, encode := range in {
		out[i] = (encode ^ muMapXorTab[i%len(muMapXorTab)]) - key
		key = encode + 0x3d
	}
	return out
}

func muEncryptMapFile(in []byte) []byte {
	out := make([]byte, len(in))
	key := byte(0x5e)
	for i, plain := range in {
		out[i] = (plain + key) ^ muMapXorTab[i%len(muMapXorTab)]
		key = out[i] + 0x3d
	}
	return out
}

func muRetagEncTerrain(raw []byte, world int) []byte {
	if world < 0 || world > 0xffff || len(raw) < 4 {
		return raw
	}
	plain := muDecryptMapFile(raw)
	if len(plain) < 4 {
		return raw
	}
	got := int(plain[0])<<8 | int(plain[1])
	if got == world {
		return raw
	}
	plain[0] = byte(world >> 8)
	plain[1] = byte(world)
	return muEncryptMapFile(plain)
}

func writeMuEncTerrainStubs(writer *zip.Writer, files []*zip.File) error {
	donor, missing := planMuEncTerrainStubs(files)
	if donor == nil || len(missing) == 0 {
		return nil
	}
	raw, err := readZipFile(donor)
	if err != nil {
		return err
	}
	encName := regexp.MustCompile(`(?i)encterrain(\d+)\.obj$`)
	for _, name := range missing {
		payload := raw
		if match := encName.FindStringSubmatch(strings.ReplaceAll(name, "\\", "/")); match != nil {
			if world, err := strconv.Atoi(match[1]); err == nil {
				payload = muRetagEncTerrain(raw, world)
			}
		}
		target, err := writer.Create(name)
		if err != nil {
			return err
		}
		if _, err := target.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

func planMuEncTerrainStubs(files []*zip.File) (*zip.File, []string) {
	type worldInfo struct {
		dir    string
		number string
		hasObj bool
		hasMap bool
	}
	worlds := map[string]*worldInfo{}
	var donor *zip.File
	donorSize := int64(-1)
	for _, file := range files {
		normalized := strings.ReplaceAll(file.Name, "\\", "/")
		dir, base := path.Split(normalized)
		worldMatch := muWorldPath.FindStringSubmatch(normalized)
		encMatch := muEncTerrainRel.FindStringSubmatch(base)
		if worldMatch == nil || encMatch == nil {
			continue
		}
		number := worldMatch[1]
		info := worlds[number]
		if info == nil {
			info = &worldInfo{dir: dir, number: number}
			worlds[number] = info
		}
		switch strings.ToLower(encMatch[2]) {
		case "obj":
			info.hasObj = true
			if file.UncompressedSize64 == 0 {
				continue
			}
			if donorSize >= 0 && int64(file.UncompressedSize64) >= donorSize {
				continue
			}
			donor = file
			donorSize = int64(file.UncompressedSize64)
		case "map":
			info.hasMap = true
		}
	}
	if donor == nil {
		return nil, nil
	}
	var missing []string
	for _, info := range worlds {
		if !info.hasMap || info.hasObj {
			continue
		}
		missing = append(missing, info.dir+"EncTerrain"+info.number+".obj")
	}
	return donor, missing
}

// copyZipFile copies an entry from the base zip into the output without
// recompressing it: CreateRaw + OpenRaw stream the already-compressed bytes
// directly. For a ~700MB client this turns a CPU-bound recompression (tens of
// seconds) into a fast I/O copy.
func copyZipFile(writer *zip.Writer, file *zip.File) error {
	header := file.FileHeader
	if file.FileInfo().IsDir() {
		_, err := writer.CreateHeader(&header)
		return err
	}

	target, err := writer.CreateRaw(&header)
	if err != nil {
		return err
	}

	source, err := file.OpenRaw()
	if err != nil {
		return err
	}

	_, err = io.Copy(target, source)
	return err
}
