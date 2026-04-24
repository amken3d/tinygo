package builder

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestBuildFSBLImageStructure sanity-checks the static fields that don't
// depend on ST's tool: every v2.3 header we emit should carry "STM2",
// version 0x00020300, option 0x80000000, binary type FSBL.
func TestBuildFSBLImageStructure(t *testing.T) {
	payload := bytes.Repeat([]byte{0xAA, 0x55}, 128) // 256 bytes, already 64-aligned
	img := buildFSBLImage(payload, 0x34180401)

	if len(img) != 0x400+len(payload) {
		t.Fatalf("image len = %d; want %d", len(img), 0x400+len(payload))
	}
	le := binary.LittleEndian
	if got := le.Uint32(img[0x000:]); got != 0x324D5453 {
		t.Errorf("magic = %#x; want STM2", got)
	}
	if got := le.Uint32(img[0x068:]); got != 0x00020300 {
		t.Errorf("header version = %#x; want 0x00020300", got)
	}
	if got := le.Uint32(img[0x06C:]); got != uint32(0x1C0+len(payload)) {
		t.Errorf("payload size = %#x; want %#x (0x1C0 + len(payload))",
			got, 0x1C0+len(payload))
	}
	if got := le.Uint32(img[0x070:]); got != 0x34180401 {
		t.Errorf("entry = %#x; want thumb bit preserved", got)
	}
	if got := le.Uint32(img[0x084:]); got != 0x80000000 {
		t.Errorf("option flags = %#x; want 0x80000000", got)
	}
	if got := le.Uint32(img[0x08C:]); got != 0x00000010 {
		t.Errorf("binary type = %#x; want 0x10 (FSBL)", got)
	}

	// Checksum = sum of 128 * (0xAA + 0x55) = 128 * 0xFF = 0x7F80.
	if got := le.Uint32(img[0x064:]); got != 0x7F80 {
		t.Errorf("checksum = %#x; want 0x7F80", got)
	}
}

// TestSignAgainstSTTool runs our signer and STMicroelectronics'
// STM32_SigningTool_CLI on the same raw payload and diffs the two outputs.
// Skipped automatically when the ST tool isn't on PATH and isn't in the
// known install locations, so CI without CubeProgrammer still passes.
func TestFSBLAgainstSTTool(t *testing.T) {
	stTool := findSTSigningTool()
	if stTool == "" {
		t.Skip("STM32_SigningTool_CLI not found — skipping byte-compat check")
	}

	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = byte(i*7 + 3)
	}
	const entry = 0x34180401

	tmp := t.TempDir()
	inBin := filepath.Join(tmp, "in.bin")
	stOut := filepath.Join(tmp, "st.bin")
	if err := os.WriteFile(inBin, payload, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(stTool,
		"-bin", inBin,
		"-nk",
		"-t", "fsbl",
		"-hv", "2.3",
		"-ep", "0x34180401",
		"-o", stOut,
		"-s",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ST tool failed: %v\n%s", err, out)
	}

	ours := buildFSBLImage(payload, entry)
	theirs, err := os.ReadFile(stOut)
	if err != nil {
		t.Fatal(err)
	}

	if len(ours) != len(theirs) {
		t.Fatalf("length mismatch: ours=%d theirs=%d", len(ours), len(theirs))
	}

	// Byte-by-byte comparison of the fields we set. ST fills some header
	// regions (random padding between 0x0A8 and 0x23F, extended block at
	// 0x240..0x3FF) with data that's either random or tool-specific, so
	// we only compare the canonical header fields plus the entire payload.
	compareOffsets := []struct {
		off, len int
		name     string
	}{
		{0x000, 4, "magic"},
		{0x004, 0x60, "signature"},
		{0x064, 4, "checksum"},
		{0x068, 4, "header-version"},
		{0x06C, 4, "payload-size"},
		{0x070, 4, "entry-point"},
		{0x078, 4, "load-address"},
		{0x084, 4, "option-flags"},
		{0x088, 4, "ext-block-size"},
		{0x08C, 4, "binary-type"},
		{0x0A0, 4, "pad-magic"},
		{0x0A4, 4, "pad-size"},
	}
	for _, f := range compareOffsets {
		if !bytes.Equal(ours[f.off:f.off+f.len], theirs[f.off:f.off+f.len]) {
			t.Errorf("field %s (offset %#x) differs:\n  ours:   %x\n  theirs: %x",
				f.name, f.off,
				ours[f.off:f.off+f.len],
				theirs[f.off:f.off+f.len])
		}
	}

	if !bytes.Equal(ours[0x400:], theirs[0x400:]) {
		t.Errorf("payload body differs")
	}
}

func findSTSigningTool() string {
	if p, err := exec.LookPath("STM32_SigningTool_CLI"); err == nil {
		return p
	}
	candidates := []string{
		"/opt/st/stm32cubeclt_1.21.0/STM32CubeProgrammer/bin/STM32_SigningTool_CLI",
		"/opt/st/stm32cubeclt_1.19.0/STM32CubeProgrammer/bin/STM32_SigningTool_CLI",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
