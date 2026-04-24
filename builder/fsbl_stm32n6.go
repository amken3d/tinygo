package builder

// This file wraps a linked ELF with a signed STM32N6 FSBL v2.3 header so the
// N6's ROM bootloader will accept, checksum-verify, and launch it from XSPI
// flash (typically 0x70000000) or AXISRAM2.
//
// Output is byte-compatible with STMicroelectronics' STM32_SigningTool_CLI
// invoked in its no-key mode:
//
//     STM32_SigningTool_CLI -bin in.bin -nk -t fsbl -hv 2.3
//
// Header layout (offsets within the 1024-byte header):
//
//     0x000  Magic "STM2"
//     0x004  Signature (96 bytes, zeros in no-key mode)
//     0x064  Checksum (sum of payload bytes, mod 2^32)
//     0x068  Header version (0x00020300 = v2.3.0)
//     0x06C  Payload size (rounded up to 64 bytes)
//     0x070  Entry point (Thumb bit set)
//     0x074  Reserved
//     0x078  Load address (0xFFFFFFFF = let ROM deduce copy target / XIP from entry-point range)
//     0x07C  Reserved
//     0x080  Image version (0)
//     0x084  Option flags (0x80000000 = no-signature)
//     0x088  Extension-header block size
//     0x08C  Binary type (0x10 = FSBL)
//     0x090  16 reserved bytes
//     0x0A0  Padding-header magic 0xFFFF5453
//     0x0A4  Padding-header size 0x1A0
//     0x0A8  Padding / reserved (zeros through 0x3FF)

import (
	"debug/elf"
	"encoding/binary"
	"os"
)

const (
	fsblHeaderSize      = 0x400
	fsblExtRegionStart  = 0x240 // start of extended/padding region within the 1024-byte header
	fsblExtRegionLen    = fsblHeaderSize - fsblExtRegionStart
	fsblPayloadAlign    = 64
	fsblHeaderMagic     = 0x324D5453 // "STM2" little-endian
	fsblHeaderVersion   = 0x00020300 // v2.3.0
	fsblOptionNoSig     = 0x80000000
	fsblExtBlockSize    = 0x000001A0
	fsblBinaryTypeFSBL  = 0x00000010
	fsblPadHeaderMagic  = 0xFFFF5453
	fsblPadHeaderSize   = 0x000001A0
	fsblLoadAddrDefault = 0xFFFFFFFF // matches ST's -nk -t fsbl default; ROM deduces copy/XIP from entry-point range
)

// signSTM32N6FSBL reads an ELF, extracts its ROM image, rounds the payload
// up to a 64-byte boundary, and writes the FSBL-signed result to outfile.
// The signature region is left zeroed (matches `-nk` on ST's signing tool).
func signSTM32N6FSBL(infile, outfile string) error {
	entry, err := readELFEntry(infile)
	if err != nil {
		return err
	}
	_, payload, err := extractROM(infile)
	if err != nil {
		return err
	}
	signed := buildFSBLImage(payload, uint32(entry))
	return os.WriteFile(outfile, signed, 0666)
}

// buildFSBLImage prepends a 1024-byte FSBL v2.3 header to payload and
// returns the combined image. Payload is padded with zeros up to a 64-byte
// boundary before checksumming (mirrors ST's `-align`/64-byte behavior).
//
// The load-address field is left as 0xFFFFFFFF — ST's no-key sign default.
// Empirically, populating it with the payload's linked address makes the N6
// ROM attempt to *write* there (failing for XIP images in read-only flash),
// whereas the sentinel lets ROM derive the destination from the entry-point
// range (SRAM → copy; XSPI → XIP).
func buildFSBLImage(payload []byte, entry uint32) []byte {
	if pad := len(payload) % fsblPayloadAlign; pad != 0 {
		payload = append(payload, make([]byte, fsblPayloadAlign-pad)...)
	}

	var checksum uint32
	for _, b := range payload {
		checksum += uint32(b)
	}

	out := make([]byte, fsblHeaderSize+len(payload))
	header := out[:fsblHeaderSize]
	le := binary.LittleEndian
	le.PutUint32(header[0x000:], fsblHeaderMagic)
	le.PutUint32(header[0x064:], checksum)
	le.PutUint32(header[0x068:], fsblHeaderVersion)
	// Image size includes the extended-header region that the ROM bootloader
	// reads alongside the payload (0x240..0x3FF, 0x1C0 bytes).
	le.PutUint32(header[0x06C:], uint32(fsblExtRegionLen+len(payload)))
	le.PutUint32(header[0x070:], entry|1)
	le.PutUint32(header[0x078:], fsblLoadAddrDefault)
	le.PutUint32(header[0x084:], fsblOptionNoSig)
	le.PutUint32(header[0x088:], fsblExtBlockSize)
	le.PutUint32(header[0x08C:], fsblBinaryTypeFSBL)
	le.PutUint32(header[0x0A0:], fsblPadHeaderMagic)
	le.PutUint32(header[0x0A4:], fsblPadHeaderSize)

	copy(out[fsblHeaderSize:], payload)
	return out
}

func readELFEntry(path string) (uint64, error) {
	f, err := elf.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return f.Entry, nil
}
