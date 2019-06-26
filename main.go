package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

const BlockSizeChunk = 512
const BlockSizeTable = 512

type Descriptor struct {
	FileName [64]byte  // filename or update name
	Reserved [6]uint32 // unused
	Index    uint32    // kind of index?

	// size of file, in update:
	// datastart + chunksize / chunk_block_size
	// = file_size in blocks � 512 bytes
	FileSize uint32

	// unknown but shared between some files
	// it seems to be infact a 128-bit structure, as
	// those data records sharing the first sig have the
	// 2nd sig also equal.
	Signature [2]uint64
	Data      [2]uint64 // unknown
}

type UpdateFile struct {
	FileDescriptor *Descriptor
	Files          []*File
	DataStart      uint32
	rawData        []byte
}

type File struct {
	Descriptor
	ChunkSize uint32
	StartOff  uint32
}

func main() {
	filePath := "/home/fionera/Projects/X32/X32_Firmware_3.11/dcp_corefs_3.11.update"

	updateFile, err := LoadUpdateFile(filePath)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Loaded File `%s`", string(updateFile.FileDescriptor.FileName[:]))


	logrus.Infof("Saving to FS")
	err = updateFile.saveToFs()
	if err != nil {
		logrus.Fatal(err)
	}
}

func LoadUpdateFile(path string) (updateFile *UpdateFile, err error) {
	updateFile = &UpdateFile{}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	updateFile.rawData = data
	reader := bytes.NewReader(updateFile.rawData)

	logrus.Debugf("Descriptor Struct is %d bytes long", binary.Size(Descriptor{}))
	logrus.Infof("Loading File Descriptor")

	descriptor, err := loadFileDescriptor(reader)
	if err != nil {
		return nil, err
	}

	updateFile.FileDescriptor = descriptor
	updateFile.DataStart = ((updateFile.FileDescriptor.Index + 3) / 4) * BlockSizeTable

	logrus.Infof("Loading Update Content")
	list, err := updateFile.loadFileList(reader)
	if err != nil {
		return nil, err
	}

	updateFile.Files = list

	return updateFile, nil
}

func (u *UpdateFile) saveToFs() (err error) {
	for _, file := range u.Files {
		fileData := u.rawData[file.StartOff:file.StartOff + file.ChunkSize]
		if len(fileData) != int(file.ChunkSize) {
			logrus.Infof("ChunkSize %d - FileDataSize %d", file.ChunkSize, len(fileData))
			return errors.New("wrong fileSize")
		}

		fileName := string(file.FileName[:])
		folderPath := path.Dir(fileName)

		fileName = path.Base(fileName)
		folderPath = path.Join("Extracted", folderPath)

		err = os.MkdirAll(folderPath, 0755)
		if err != nil {
			return err
		}


		err = ioutil.WriteFile(path.Join(folderPath, strings.Trim(fileName,"\x00")), fileData, 0755)
		if err != nil {
			return err
		}
	}

	return nil
}

func loadFileDescriptor(reader io.Reader) (descriptor *Descriptor, err error) {
	descriptor = &Descriptor{}
	err = binary.Read(reader, binary.LittleEndian, descriptor)
	if err != nil {
		return nil, err
	}

	if descriptor.Index < 1 || descriptor.Index > 5000 {
		logrus.Fatal(errors.New("invalid Data"))
	}

	logrus.Debugf("Filename is `%s`", string(descriptor.FileName[:]))
	logrus.Debugf("Filesize is `%d`", descriptor.FileSize)

	return descriptor, nil
}

func (u *UpdateFile) loadFileList(reader io.Reader) (fileList []*File, err error) {
	fileList = make([]*File, u.FileDescriptor.Index)

	var sizeChunk uint32
	var sizeSum uint32
	descriptor := Descriptor{}
	for i := 0; i < int(u.FileDescriptor.Index); i++ {
		err := binary.Read(reader, binary.LittleEndian, &descriptor)
		if err != nil {
			return nil, err
		}

		file := File{descriptor, 0, 0}
		file.ChunkSize = ((file.FileSize + BlockSizeChunk - 1) / BlockSizeChunk) * BlockSizeChunk
		file.StartOff = sizeChunk + u.DataStart

		sizeChunk += file.ChunkSize
		sizeSum += file.FileSize

		logrus.Debugf("`%s`- %s at %d",
			string(file.FileName[:]),
			humanize.Bytes(uint64(file.FileSize)),
			file.StartOff,
		)

		fileList[i] = &file
	}

	//logrus.Info(u.FileDescriptor.FileSize*BlockSizeChunk)
	//logrus.Info(sizeChunk+u.DataStart)
	if u.FileDescriptor.FileSize*BlockSizeChunk != sizeChunk+u.DataStart {
		logrus.Info("invalid Filesize")
		//return nil, errors.New("invalid Filesize")
	}

	return fileList, nil
}
