package main

import (
	"bytes"
	"debug/pe"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	_ "github.com/Binject/debug/pe"
)

var (
	file *string
)

type (
	FileData struct {
		headerneedle   *regexp.Regexp
		filedata       []byte
		reader         *bytes.Reader
		currentchunk   []byte
		offsets        [][]int
		chunkincrement int
	}
)

func (d *FileData) GenerateOffsets() (*FileData, error) {
	d.offsets = d.headerneedle.FindAllIndex(d.filedata, -1)
	if len(d.offsets) < 1 {
		return nil, errors.New("no files found embedded")
	}

	return d, nil
}

func CreateStructure(contents []byte) *FileData {
	return &FileData{
		headerneedle: regexp.MustCompile(`This program cannot be run in DOS mode`),
		filedata:     contents,
	}
}

func (data *FileData) GenerateBlob(prefix string) {
	if _, err := os.Stat("CARVED_FILES"); os.IsNotExist(err) {
		err = os.Mkdir("CARVED_FILES", 0750)
		if err != nil {
			log.Fatal(err)
		}
	}

	filename := fmt.Sprintf("CARVED_FILES/%s_%04d.bin", prefix, data.chunkincrement)
	file, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer fmt.Printf("Done writing: %s\n\n", filename)
	defer file.Close()

	file.Write(data.currentchunk)
}

func init() {
	file = flag.String("file", "", "File to parse.")
	flag.Parse()
}

func main() {
	if *file == "" {
		log.Fatal("You must provide a filename to parse")
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		log.Fatal(err)
	}

	structure, err := CreateStructure(data).GenerateOffsets()
	if err != nil {
		log.Fatal(err)
	}

	for _, item := range structure.offsets {
		mzheader := regexp.MustCompile("MZ")
		if mzheader.Match(structure.filedata[item[0]-78 : item[1]-78]) {
			fmt.Printf("Carving file at offset: %d\n", item[0]-78)
			structure.reader = bytes.NewReader(structure.filedata[item[0]-78:])
			f, err := pe.NewFile(structure.reader)
			if err != nil {
				log.Fatal(err)
			}

			size := 0

			switch dtype := f.OptionalHeader.(type) {
			case *pe.OptionalHeader64:
				size += int(dtype.SizeOfHeaders)
			case *pe.OptionalHeader32:
				size += int(dtype.SizeOfHeaders)
			default:
				break
			}

			for _, section := range f.Sections {
				size += int(section.Size)
			}

			fmt.Printf("Assumed size: %d\n", size)

			structure.currentchunk = append(structure.currentchunk, structure.filedata[item[0]-78:item[0]-78+size]...)
			structure.chunkincrement += 1

			if f.Characteristics&pe.IMAGE_FILE_DLL == pe.IMAGE_FILE_DLL {
				structure.GenerateBlob(strings.Replace(fmt.Sprintf("OFFSET_%012d_TO_%010d-DLL", item[0]-78, item[0]-78+size-1), " ", "", -1))
			} else {
				structure.GenerateBlob(strings.Replace(fmt.Sprintf("OFFSET_%012d-%012d-EXE", item[0]-78, item[0]-78+size-1), " ", "", -1))
			}
		}
	}
}
