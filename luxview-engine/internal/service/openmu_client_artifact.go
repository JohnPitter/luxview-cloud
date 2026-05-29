package service

import (
	"archive/zip"
	"encoding/xml"
	"io"
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
	_, err = io.WriteString(configWriter, BuildOpenMULauncherConfig(opts.ServerName, opts.ServerIP, opts.GamePort))
	return err
}

func copyZipFile(writer *zip.Writer, file *zip.File) error {
	header := file.FileHeader
	target, err := writer.CreateHeader(&header)
	if err != nil {
		return err
	}
	if file.FileInfo().IsDir() {
		return nil
	}

	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	_, err = io.Copy(target, source)
	return err
}
