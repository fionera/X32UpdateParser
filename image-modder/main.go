package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/snksoft/crc"
)

func main() {
	filePath := "/home/fionera/Projects/X32/firmware/Images/3.11/X32C.raw"

	file, err := os.Open(filePath)
	if err != nil {
		logrus.Fatal(err)
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		logrus.Fatal(err)
	}

	reader := bytes.NewReader(data)
	_, _ = reader.Seek(0x200, 0)

	cfgString := ReadCString(reader)
	config := ParseConfig(cfgString)

	config["SN"] = "\nHIER\nKOENNTE\nIHRE\nWERBUNG\nSTEHEN\n"
	config["MAC"] = "020023421337"
	config["NOBOOT"] = "Y"

	dst := make([]byte, len(data))
	copy(dst, data)

	overrideBytes(dst, []byte(config.String()), 0x200)

	ioutil.WriteFile("out.raw", dst, 0755)
	//logrus.Info(cfgString)
	//logrus.Info(config)
}

func overrideBytes(dst []byte, src []byte, offset int) {
	for i := offset; i < len(src)+offset; i++ {
		dst[i] = src[i-offset]
	}
}

func ReadCString(r *bytes.Reader) string {
	var data []byte
	for {
		b, _ := r.ReadByte()
		if b == 0x00 {
			break
		}

		data = append(data, b)
	}
	return string(data)
}

type BootConfig map[string]string

func ParseConfig(config string) BootConfig {
	parts := strings.Split(config, ":")

	var cfg = make(BootConfig)
	for _, part := range parts[2:] {
		split := strings.Split(part, "=")
		cfg[split[0]] = split[1]
	}

	return cfg
}

func (bc BootConfig) String() string {
	var parts []string
	for k, v := range bc {

		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}

	cfgString := strings.Join(parts, ":")
	crc16 := crc.NewHash(crc.XMODEM)
	_, _ = crc16.Write([]byte(cfgString))

	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, crc16.CRC16())
	enc := hex.EncodeToString(b)

	return fmt.Sprintf(":CFG%s:", strings.ToUpper(enc)) + strings.Join(parts, ":")
}
