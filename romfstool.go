package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

const (
	FHDR_NEXT_MASK = 0xfffffff0
	FHDR_EXEC_MASK = 0x00000008
	FHDR_TYPE_MASK = 0x00000007
)

const (
	FTYPE_LINK = iota
	FTYPE_DIR
	FTYPE_FILE
	FTYPE_SYMLINK
	FTYPE_BLKDEV
	FTYPE_CHRDEV
	FTYPE_SOCKET
	FTYPE_FIFO
)

var (
	exitCode    = 0
	magic0      = [8]byte{0x2d, 0x72, 0x6f, 0x6d, 0x31, 0x66, 0x73, 0x2d}
	magic1      = [8]byte{0x6d, 0x6f, 0x72, 0x2d, 0x2d, 0x73, 0x66, 0x31}
	ftypeSymbol = []string{"", "/", "", "@", "*", "*", "=", "="}
)

var (
	flagExtract = flag.String("x", "", "extract root dir")
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

type romfsInfo struct {
	base int64
	swap bool
	size uint32
}

type fileInfo struct {
	pos      int64
	next     int64
	size     int64
	bodypos  int64
	ftype    uint32
	info     uint32
	name     string
	checksum uint32
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: romfstool [-x DIR] <ROMFS FILE>\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if len(flag.Args()) < 1 {
		flag.Usage()
	} else {
		romfstool(flag.Arg(0))
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

func romfstool(filename string) {
	file, err := os.Open(filename)
	chkerr(err, "open")
	defer file.Close()

	romfs := searchMagic(file)
	if romfs.base < 0 {
		fmt.Println("Not romfs")
		return
	}

	volname := readString(file, romfs.swap)
	fmt.Printf("volume name: %s\n", volname)
	fmt.Printf("size: %d\n", romfs.size)
	if romfs.base > 0 {
		fmt.Printf("offset: 0x%x\n", romfs.base)
	}
	if romfs.swap {
		fmt.Println("endian: swapped")
	}
	if *flagExtract != "" {
		fmt.Printf("extract: %s\n", *flagExtract)
	}

	fmt.Printf("\n    offset      size  filename\n")
	readDir(file, romfs, "")
}

func searchMagic(file *os.File) romfsInfo {
	for {
		pos, _ := file.Seek(0, 1)

		var header romfsHeader
		err := binary.Read(file, binary.BigEndian, &header)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		chkerr(err, "search magic")

		if header.Magic == magic0 {
			return romfsInfo{
				base: pos,
				swap: false,
				size: header.Size,
			}
		} else if header.Magic == magic1 {
			return romfsInfo{
				base: pos,
				swap: true,
				size: swapWord(header.Size),
			}
		}
	}

	return romfsInfo{
		base: -1,
	}
}

func getFileInfo(file *os.File, romfs romfsInfo, pos int64) fileInfo {
	_, err := file.Seek(romfs.base+pos, 0)
	chkerr(err, "file.Seek")

	var header romfsFileHeader
	if romfs.swap {
		err = binary.Read(file, binary.LittleEndian, &header)
	} else {
		err = binary.Read(file, binary.BigEndian, &header)
	}
	chkerr(err, "binary.Read")

	filename := readString(file, romfs.swap)
	hdrlen := (16 + len(filename) + 1 + 15) & ^15

	return fileInfo{
		pos:      pos,
		next:     int64(header.Next & FHDR_NEXT_MASK),
		size:     int64(header.Size),
		bodypos:  pos + int64(hdrlen),
		ftype:    header.Next & FHDR_TYPE_MASK,
		info:     header.Info,
		name:     filename,
		checksum: header.Checksum,
	}
}

func readDir(file *os.File, romfs romfsInfo, path string) {
	for {
		pos, err := file.Seek(0, 1)
		chkerr(err, "file.Seek")
		pos -= romfs.base

		finfo := getFileInfo(file, romfs, pos)

		fstr := finfo.name + ftypeSymbol[finfo.ftype]

		if finfo.ftype == FTYPE_LINK {
			link := int64(finfo.info)
			linfo := getFileInfo(file, romfs, link)
			fstr += fmt.Sprintf(" -> %s%s",
				linfo.name, ftypeSymbol[linfo.ftype])
		}

		if finfo.ftype == FTYPE_FILE && *flagExtract != "" {
			extractFile(file, romfs, finfo, path)
		}

		fmt.Printf("0x%08x  %8d  %s/%s\n",
			finfo.pos, finfo.size, path, fstr)

		// traverse sub directory
		if finfo.ftype == FTYPE_DIR && finfo.name != "." {
			file.Seek(romfs.base+int64(finfo.info), 0)
			readDir(file, romfs, path+"/"+finfo.name)
		}

		if finfo.next == 0 {
			break
		}

		file.Seek(romfs.base+finfo.next, 0)
	}
}

func extractFile(file *os.File, romfs romfsInfo, finfo fileInfo, path string) {
	rootdir := *flagExtract
	if path != "" {
		rootdir += "/" + path
	}

	err := os.MkdirAll(rootdir, 0755)
	chkerr(err, "os.MkdirAll")

	_, err = file.Seek(romfs.base+finfo.bodypos, 0)
	chkerr(err, "file.Seek")

	fw, err := os.Create(rootdir + "/" + finfo.name)
	chkerr(err, "os.Create")
	defer fw.Close()

	buf := make([]byte, 4096)
	length := finfo.size
	for length > 0 {
		cnt, err := file.Read(buf)
		chkerr(err, "Read")

		if romfs.swap {
			swapBinary(buf)
		}

		if length < int64(cnt) {
			cnt = int(length)
		}
		length -= int64(cnt)

		fw.Write(buf[:cnt])
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

func swapWord(data uint32) uint32 {
	return (data&0x000000ff)<<24 |
		(data&0x0000ff00)<<8 |
		(data&0x00ff0000)>>8 |
		(data&0xff000000)>>24
}
