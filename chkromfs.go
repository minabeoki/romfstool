package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"
)

const (
	FHDR_NEXT_MASK = 0xfffffff0
	FHDR_EXEC_MASK = 0x00000008
	FHDR_TYPE_MASK = 0x00000007
	FTYPE_LINK     = 0
	FTYPE_DIR      = 1
	FTYPE_FILE     = 2
)

var (
	exitCode = 0
	magic0   = [8]byte{0x2d, 0x72, 0x6f, 0x6d, 0x31, 0x66, 0x73, 0x2d}
	magic1   = [8]byte{0x6d, 0x6f, 0x72, 0x2d, 0x2d, 0x73, 0x66, 0x31}
)

type romfsHeader struct {
	Magic    [8]byte
	Size     uint32
	Checksum uint32
	// volume name (variable length)
}

type romfsFileHeader struct {
	Next     uint32
	Info     uint32
	Size     uint32
	Checksum uint32
	// file name (variable length)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: chkromfs FILE")
	} else {
		chkromfs(os.Args[1])
	}

	os.Exit(exitCode)
}

func chkerr(err error, msg ...string) {
	if err != nil {
		if len(msg) > 0 {
			log.Fatal(fmt.Sprintf("%s: %s", msg[0], err))
		} else {
			log.Fatal(err)
		}
	}
}

func chkromfs(filename string) {
	fmt.Printf("romfs: %s\n", filename)
	file, err := os.Open(filename)
	chkerr(err, "open")
	defer file.Close()

	var header romfsHeader
	err = binary.Read(file, binary.BigEndian, &header)
	chkerr(err, "binary.Read")

	//fmt.Printf("%x\n", header.Magic)
	//fmt.Printf("%08x\n", header.Checksum)
	//fmt.Printf("%08x\n", header.Size)

	// check magic and swap

	swap := false
	if header.Magic == magic0 {
	} else if header.Magic == magic1 {
		fmt.Println("swapped binary")
		swap = true
	} else {
		fmt.Println("Not romfs")
		return
	}

	volname := readString(file, swap)
	fmt.Println("volume:", volname)

	readDir(file, swap, 0)
}

func readDir(file *os.File, swap bool, indent int) {
	for {
		pos, err := file.Seek(0, 1)

		var header romfsFileHeader
		if swap {
			err = binary.Read(file, binary.LittleEndian, &header)
		} else {
			err = binary.Read(file, binary.BigEndian, &header)
		}
		chkerr(err, "binary.Read")

		next := header.Next & FHDR_NEXT_MASK
		ftype := header.Next & FHDR_TYPE_MASK
		info := header.Info
		fname := readString(file, swap)
		pre := strings.Repeat(" ", indent)
		fmt.Printf("%s%x: nx:%x ft:%x info:%x %s\n",
			pre, pos, next, ftype, info, fname)

		if ftype == FTYPE_DIR && fname != "." {
			file.Seek(int64(info), 0)
			readDir(file, swap, indent+1)
		}

		if next == 0 {
			break
		}

		file.Seek(int64(next), 0)
	}
}

func readString(file *os.File, swap bool) string {
	data := make([]byte, 256)
	count, err := file.Read(data)
	chkerr(err)
	if swap {
		swapBinary(data)
	}

	length := bytes.IndexByte(data, 0x00)
	if length < 0 {
		file.Seek(int64(-count), 1)
		return ""
	}

	str := string(data[0:length])

	aligned := (len(str) + 1 + 15) & 0xfffffff0
	file.Seek(int64(aligned-count), 1)

	return str
}

func swapBinary(buf []byte) {
	for i := 0; i < len(buf); i += 4 {
		buf[i+0], buf[i+1], buf[i+2], buf[i+3] =
			buf[i+3], buf[i+2], buf[i+1], buf[i+0]
	}
}
