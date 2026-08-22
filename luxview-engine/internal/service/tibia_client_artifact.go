package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// tibiaInitLuaPath é o arquivo de configuração do OTClient que contém a lista
// de servidores (Servers_init). O base zip tem um placeholder com o host local;
// o artifact injeta o IP público do servidor na porta de login HTTP (8088).
const tibiaInitLuaPath = "init.lua"

// tibiaLoginHostPlaceholder é o host presente no init.lua do client base. O
// artifact o substitui pelo IP público do servidor antes de servir o zip.
const tibiaLoginHostPlaceholder = "127.0.0.1:8088"

type TibiaClientOptions struct {
	ServerName string
	ServerIP   string
	LoginPort  int // porta HTTP do login-server dentro do container
}

// buildTibiaInitLua injeta o servidor no init.lua base do OTClient.
func buildTibiaInitLua(base []byte, opts TibiaClientOptions) []byte {
	loginHost := opts.ServerIP + ":" + itoa(opts.LoginPort)
	return bytes.ReplaceAll(base, []byte(tibiaLoginHostPlaceholder), []byte(loginHost))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// isTibiaInitLua normaliza o nome do arquivo (zips criados via bsdtar costumam
// prefixar os paths com "./") antes de comparar com o caminho do init.lua.
func isTibiaInitLua(name string) bool {
	name = strings.TrimPrefix(name, "./")
	return strings.EqualFold(name, tibiaInitLuaPath)
}

// WriteTibiaClientZip streams o client OTClient do base zip com o init.lua
// apontando para o login HTTP do servidor (http://<ip>:<porta>/login). Arquivos
// grandes (assets/sprites) são copiados sem recompressão (CreateRaw/OpenRaw).
func WriteTibiaClientZip(base io.ReaderAt, size int64, out io.Writer, opts TibiaClientOptions) error {
	if opts.LoginPort == 0 {
		opts.LoginPort = 8088
	}
	if opts.ServerIP == "" {
		opts.ServerIP = "127.0.0.1"
	}

	reader, err := zip.NewReader(base, size)
	if err != nil {
		return err
	}

	writer := zip.NewWriter(out)
	defer writer.Close()

	for _, file := range reader.File {
		if isTibiaInitLua(file.Name) {
			if err := writePatchedInitLua(writer, file, opts); err != nil {
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

// WriteTibiaClientPatch writes only init.lua — the files the engine actually
// rewrites. First install still uses the static base zip; later updates do not.
func WriteTibiaClientPatch(base io.ReaderAt, size int64, out io.Writer, opts TibiaClientOptions) error {
	if opts.LoginPort == 0 {
		opts.LoginPort = 8088
	}
	if opts.ServerIP == "" {
		opts.ServerIP = "127.0.0.1"
	}

	reader, err := zip.NewReader(base, size)
	if err != nil {
		return err
	}

	writer := zip.NewWriter(out)
	defer writer.Close()

	found := false
	for _, file := range reader.File {
		if !isTibiaInitLua(file.Name) {
			continue
		}
		if err := writePatchedInitLua(writer, file, opts); err != nil {
			return err
		}
		found = true
	}
	if !found {
		return fmt.Errorf("tibia client zip does not contain %s", tibiaInitLuaPath)
	}
	return nil
}

func writePatchedInitLua(writer *zip.Writer, file *zip.File, opts TibiaClientOptions) error {
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	content, err := io.ReadAll(source)
	if err != nil {
		return err
	}

	header := file.FileHeader
	header.Method = zip.Deflate
	header.SetModTime(file.ModTime())
	target, err := writer.CreateHeader(&header)
	if err != nil {
		return err
	}
	_, err = target.Write(buildTibiaInitLua(content, opts))
	return err
}
