package omapnative

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

// TestExecutableKeyProbe is an opt-in forensic check for a precomputed OMAP
// database key. It only reads the executable named by OMAP_KEY_PROBE.
func TestExecutableKeyProbe(t *testing.T) {
	path := os.Getenv("OMAP_KEY_PROBE")
	if path == "" {
		t.Skip("OMAP_KEY_PROBE not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := [8]byte{0x6b, 0xf5, 0x48, 0x0d, 0x43, 0x28, 0xf4, 0x5d}
	var zero [8]byte
	for offset := 0; offset+16 <= len(data); offset++ {
		var keys [52]uint16
		expandIDEAKey(&keys, data[offset:offset+16])
		got := encryptIDEABlock(zero, &keys)
		if bytes.Equal(got[:], want[:]) {
			t.Fatalf("candidate raw IDEA key at file offset 0x%x: %x", offset, data[offset:offset+16])
		}
	}
	fmt.Fprintf(os.Stderr, "no raw IDEA key in %s\n", path)
}
