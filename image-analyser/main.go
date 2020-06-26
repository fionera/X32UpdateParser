package main

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
)

type Pointer struct {
	Addr uint16
	_    uint16
}

type FLASH_HDR_T struct {
	App_code_jump_vector Pointer
	App_code_barker      uint32
	App_code_csf         uint32
	Dcd_ptr_ptr          Pointer // Pointer to DCD_T
	Super_root_key       uint32  // Pointer to hab_rsa_public_key
	Dcd_ptr              Pointer // Pointer to DCD_T
	App_dest_ptr         Pointer
}

type hab_rsa_public_key struct {
	Rsa_exponent  []uint8 /* RSA public exponent */
	Rsa_modulus   uint8   /* RSA modulus pointer */
	Exponent_size uint16  /* Exponent size in bytes */
	Modulus_size  uint16  /* Modulus size in bytes*/
	Init_flag     bool    /* Indicates if key initialized */
}

type DCD_T struct {
	Preamble DCD_PREAMBLE_T /* Preamble */
	/* Type / Address / data elements */
	Type_addr_data []DCD_TYPE_ADDR_DATA_T /*where count would be some hardcoded value less than 60*/
}

type DCD_PREAMBLE_T struct {
	Barker uint32 /* Barker for sanity check */
	Length uint32 /* Device configuration structure length (not including preamble) */
}
type DCD_TYPE_ADDR_DATA_T struct {
	Type uint32 /* Type of pointer (byte=0x1, halfword=0x2, word=0x4) */
	Addr uint32 /* Address to write to */
	Data uint32 /* Data to write */
}

/* Flash Header Structure */
type FLASH_CFG_PARMS_T struct {
	Length uint32 /* Length of data to be read */
}

type MBR struct {
	Partitions [4]Partition
}

type CHS struct {
	Cylinder byte
	Head     byte
	Sector   byte
}

type Partition struct {
	Boot           byte
	StartingCHS    CHS
	PartitionType  byte
	EndingCHS      CHS
	StartingSector uint32
	PartitionSize  uint32
}

const SectorSize = 512
const TrackSize = 63

func main() {
	//filePath := "/home/fionera/Projects/X32/firmware/3.11/X32C.raw"
	filePath := "/home/fionera/Projects/X32/firmware/Images/3.11/X32C.raw"
	//filePath := "/home/fionera/Projects/X32/firmware/3.11/sd2.raw"

	file, err := os.Open(filePath)
	if err != nil {
		logrus.Fatal(err)
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		logrus.Fatal(err)
	}

	headerReader := bytes.NewReader(data)
	_, _ = headerReader.Seek(0x1BE, 0)

	var mbr MBR
	err = binary.Read(headerReader, binary.LittleEndian, &mbr)
	if err != nil {
		logrus.Fatal(err)
	}

	_, _ = headerReader.Seek(0x200, 0)

	deviceInfoString := ReadCString(headerReader)
	rawInfo := deviceInfoString[9:]
	logrus.Println(string(rawInfo))

	_, _ = headerReader.Seek(0x400, 0)

	var hdr FLASH_HDR_T
	err = binary.Read(headerReader, binary.LittleEndian, &hdr)
	if err != nil {
		logrus.Fatal(err)
	}

	if hdr.App_code_barker != 0x000000B1 {
		logrus.Fatal("Invalid Barker")
	}

	if hdr.Super_root_key != 0x0 {
		logrus.Fatal("Encrypted!")
	}

	if hdr.App_code_csf != 0x0 {
		logrus.Fatal("HAB Cert not 0x0")
	}

	var preamble DCD_PREAMBLE_T
	err = binary.Read(headerReader, binary.LittleEndian, &preamble)
	if err != nil {
		logrus.Fatal(err)
	}

	if preamble.Barker != 0xB17219E9 {
		logrus.Fatal("Invalid Barker")
	}

	arrLen := int(preamble.Length) / binary.Size(DCD_TYPE_ADDR_DATA_T{})
	logrus.Infof("Calculated arrLen: %d", arrLen)

	dcdTypeDataArr := make([]DCD_TYPE_ADDR_DATA_T, arrLen)
	for i := 0; i < len(dcdTypeDataArr); i++ {
		var dest DCD_TYPE_ADDR_DATA_T
		err = binary.Read(headerReader, binary.LittleEndian, &dest)
		if err != nil {
			logrus.Fatal(err)
		}

		dcdTypeDataArr[i] = dest
	}

	dcd := DCD_T{
		Preamble:       preamble,
		Type_addr_data: dcdTypeDataArr,
	}

	var flashCfgParams FLASH_CFG_PARMS_T
	err = binary.Read(headerReader, binary.LittleEndian, &flashCfgParams)
	if err != nil {
		logrus.Fatal(err)
	}

	dcd = dcd // Dont annoy me go

	logrus.Infof("Copy dst: %d", hdr.App_dest_ptr.Addr)
	logrus.Infof("Copy size: %d", flashCfgParams.Length)
	bootLoader := make([]byte, flashCfgParams.Length)
	_, err = headerReader.Read(bootLoader)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info(hdr.App_code_jump_vector.Addr)
	bootLoader = bootLoader[hdr.App_code_jump_vector.Addr:]
	logrus.Infof("Cutted size: %d", len(bootLoader))

	//bootLoaderStart := 0x400 + binary.Size(hdr) + binary.Size(DCD_PREAMBLE_T{}) + (binary.Size(DCD_TYPE_ADDR_DATA_T{}) * len(dcd.Type_addr_data)) + binary.Size(FLASH_CFG_PARMS_T{})

	logrus.Infof("%x", int(hdr.App_code_jump_vector.Addr))

	//logrus.Info(bootLoader)
}

func ReadCString(r *bytes.Reader) []byte {
	var data []byte
	for {
		b, _ := r.ReadByte()
		if b == 0x00 {
			break
		}

		data = append(data, b)
	}
	return data
}
