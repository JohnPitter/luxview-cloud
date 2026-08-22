package service

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"path"
	"regexp"
	"strings"
)

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

// WriteOpenMUClientPatch writes launcher.config plus EncTerrain.obj stubs for
// Season 9 worlds that shipped without object files.
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
	return writeMuEncTerrainStubs(writer, reader.File)
}

var (
	muWorldPath     = regexp.MustCompile(`(?i)(?:^|/)world(\d+)(?:/|$)`)
	muEncTerrainRel = regexp.MustCompile(`(?i)^encterrain(\d+)\.(obj|map)$`)
)

func writeMuEncTerrainStubs(writer *zip.Writer, files []*zip.File) error {
	donor, missing := planMuEncTerrainStubs(files)
	if donor == nil || len(missing) == 0 {
		return nil
	}
	raw, err := readZipFile(donor)
	if err != nil {
		return err
	}
	for _, name := range missing {
		target, err := writer.Create(name)
		if err != nil {
			return err
		}
		if _, err := target.Write(raw); err != nil {
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
