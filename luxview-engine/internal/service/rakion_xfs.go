package service

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// XFS2 ("Xenesis2") container parsing/repacking — enough to rewrite the small,
// single-chunk files Rakion ships (e.g. NyxLauncherEnc.xfs, which holds the
// launcher's fetch/auto-download URLs). Port of tools/xfs_repack.py.
//
// Layout: [i32 start][file blocks][1B headZSize][zlib head][3B ftSize][zlib filetable]
//   head:  "XFS2" + i32 version + i32 count + i32 validation + i32 start + tailString
//   filetable entry: name[112] + i32 foff + i32 comp + i32 uc + i32 cs
//   file block (single chunk): [u16 UCSize][0x80][u24 zlen][u16 cksum] + zlib(content)

type xfsFile struct {
	name    [112]byte
	foff    int32
	comp    int32
	uc      int32
	cs      int32
	content []byte // decompressed content (filled on parse / used on build)
}

func zlibInflate(b []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func zlibDeflate(b []byte, level int) []byte {
	var buf bytes.Buffer
	w, _ := zlib.NewWriterLevel(&buf, level)
	_, _ = w.Write(b)
	_ = w.Close()
	return buf.Bytes()
}

// parseXFS2 decodes a container into its file list (with decompressed content).
func parseXFS2(d []byte) (version, validation int32, tail []byte, files []*xfsFile, err error) {
	if len(d) < 8 {
		return 0, 0, nil, nil, fmt.Errorf("xfs: too short")
	}
	start := int(int32(binary.LittleEndian.Uint32(d[0:4])))
	if start <= 0 || start >= len(d) {
		return 0, 0, nil, nil, fmt.Errorf("xfs: bad start offset %d", start)
	}
	p := start
	zsize := int(d[p])
	p++
	head, err := zlibInflate(d[p : p+zsize])
	if err != nil {
		return 0, 0, nil, nil, fmt.Errorf("xfs: head inflate: %w", err)
	}
	p += zsize
	infosz := int(d[p]) | int(d[p+1])<<8 | int(d[p+2])<<16
	p += 3
	info, err := zlibInflate(d[p : p+infosz])
	if err != nil {
		return 0, 0, nil, nil, fmt.Errorf("xfs: filetable inflate: %w", err)
	}
	if len(head) < 20 || string(head[0:4]) != "XFS2" {
		return 0, 0, nil, nil, fmt.Errorf("xfs: bad magic")
	}
	version = int32(binary.LittleEndian.Uint32(head[4:8]))
	count := int(int32(binary.LittleEndian.Uint32(head[8:12])))
	validation = int32(binary.LittleEndian.Uint32(head[12:16]))
	tail = head[20:]

	for i := 0; i < count; i++ {
		base := i * 128
		if base+128 > len(info) {
			return 0, 0, nil, nil, fmt.Errorf("xfs: filetable truncated")
		}
		f := &xfsFile{}
		copy(f.name[:], info[base:base+112])
		f.foff = int32(binary.LittleEndian.Uint32(info[base+112 : base+116]))
		f.comp = int32(binary.LittleEndian.Uint32(info[base+116 : base+120]))
		f.uc = int32(binary.LittleEndian.Uint32(info[base+120 : base+124]))
		f.cs = int32(binary.LittleEndian.Uint32(info[base+124 : base+128]))
		// Single-chunk block: 8-byte header then zlib(content).
		bstart := int(f.foff) + 8
		bend := int(f.foff) + int(f.cs)
		if bstart > len(d) || bend > len(d) || bstart > bend {
			return 0, 0, nil, nil, fmt.Errorf("xfs: block out of range")
		}
		content, err := zlibInflate(d[bstart:bend])
		if err != nil {
			return 0, 0, nil, nil, fmt.Errorf("xfs: content inflate: %w", err)
		}
		f.content = content
		files = append(files, f)
	}
	return version, validation, tail, files, nil
}

func makeXFSBlock(content []byte) []byte {
	zc := zlibDeflate(content, 6)
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, uint16(len(content)))
	b.WriteByte(0x80)
	b.WriteByte(byte(len(zc)))
	b.WriteByte(byte(len(zc) >> 8))
	b.WriteByte(byte(len(zc) >> 16))
	b.Write([]byte{0x00, 0x00}) // cksum (not validated by the game)
	b.Write(zc)
	return b.Bytes()
}

// buildXFS2 repacks the file list (using each file's current content).
func buildXFS2(version, validation int32, tail []byte, files []*xfsFile) ([]byte, error) {
	out := make([]byte, 4) // placeholder start offset
	type rec struct {
		name               [112]byte
		foff, comp, uc, cs int32
	}
	var recs []rec
	for _, f := range files {
		block := makeXFSBlock(f.content)
		foff := int32(len(out))
		out = append(out, block...)
		recs = append(recs, rec{f.name, foff, f.comp, int32(len(f.content)), int32(len(block))})
	}
	start := int32(len(out))

	var head bytes.Buffer
	head.WriteString("XFS2")
	_ = binary.Write(&head, binary.LittleEndian, version)
	_ = binary.Write(&head, binary.LittleEndian, int32(len(files)))
	_ = binary.Write(&head, binary.LittleEndian, validation)
	_ = binary.Write(&head, binary.LittleEndian, start)
	head.Write(tail)
	headZ := zlibDeflate(head.Bytes(), 9)
	if len(headZ) >= 256 {
		return nil, fmt.Errorf("xfs: head zlib too big (%d)", len(headZ))
	}

	var ft bytes.Buffer
	for _, r := range recs {
		ft.Write(r.name[:])
		_ = binary.Write(&ft, binary.LittleEndian, r.foff)
		_ = binary.Write(&ft, binary.LittleEndian, r.comp)
		_ = binary.Write(&ft, binary.LittleEndian, r.uc)
		_ = binary.Write(&ft, binary.LittleEndian, r.cs)
	}
	ftZ := zlibDeflate(ft.Bytes(), 9)

	binary.LittleEndian.PutUint32(out[0:4], uint32(start))
	out = append(out, byte(len(headZ)))
	out = append(out, headZ...)
	out = append(out, byte(len(ftZ)), byte(len(ftZ)>>8), byte(len(ftZ)>>16))
	out = append(out, ftZ...)
	return out, nil
}

// rewriteRakionLauncherEnc rewrites the fetch/auto-download host inside
// NyxLauncherEnc.xfs: Url_Fetch points at the auth host (so Traefik routes by
// Host) and Ip points at the raw server IP (broker, direct TCP).
func rewriteRakionLauncherEnc(data []byte, authHost, serverIP string) ([]byte, error) {
	version, validation, tail, files, err := parseXFS2(data)
	if err != nil {
		return nil, err
	}
	if serverIP == "" {
		serverIP = authHost
	}
	for _, f := range files {
		s := string(f.content)
		s = strings.ReplaceAll(s, "http://"+rakionDevHost+"/", "http://"+authHost+"/")
		s = strings.ReplaceAll(s, "Ip="+rakionDevHost, "Ip="+serverIP)
		f.content = []byte(s)
	}
	return buildXFS2(version, validation, tail, files)
}

func isRakionLauncherEnc(name string) bool {
	return strings.EqualFold(baseName(name), "nyxlauncherenc.xfs")
}
