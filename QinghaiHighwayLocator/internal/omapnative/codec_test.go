package omapnative

import (
	"bytes"
	"crypto/md5"
	"os"
	"testing"
)

func TestDatabaseCodecRoundTrip(t *testing.T) {
	plain := make([]byte, pageSize*2)
	copy(plain, []byte("SQLite format 3\x00"))
	for i := len("SQLite format 3\x00"); i < len(plain); i++ {
		plain[i] = byte(i * 31)
	}
	cipher, err := EncodeDatabase(plain, "ovital318uinkme")
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeDatabase(cipher, "ovital318uinkme")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatal("database codec did not round-trip")
	}
}

func TestInstalledDatabaseHeaderKeystream(t *testing.T) {
	key := md5.Sum([]byte("ovital318uinkme"))
	var schedule [52]uint16
	expandIDEAKey(&schedule, key[:])
	got := encryptIDEABlock([8]byte{}, &schedule)
	// Derived from the untouched oobj.odb page 1 and the immutable SQLite
	// header: ciphertext XOR plaintext XOR OMAP's 0x8b page mask.
	want := [8]byte{0x6b, 0xf5, 0x48, 0x0d, 0x43, 0x28, 0xf4, 0x5d}
	if got != want {
		t.Fatalf("OMAP header keystream mismatch: got %x want %x", got, want)
	}
}

// Set OMAP_CODEC_PROBE to an OMAP .odb backup to independently validate the
// installed-version codec without ever touching its live data file.
func TestInstalledBackupWhenConfigured(t *testing.T) {
	path := os.Getenv("OMAP_CODEC_PROBE")
	if path == "" {
		t.Skip("OMAP_CODEC_PROBE not set")
	}
	encrypted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecodeDatabase(encrypted, "ovital318uinkme")
	if err != nil {
		t.Fatal(err)
	}
	if string(plain[:16]) != "SQLite format 3\x00" {
		t.Fatalf("decoded database has no SQLite header: %x", plain[:16])
	}
	reencoded, err := EncodeDatabase(plain, "ovital318uinkme")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reencoded, encrypted) {
		t.Fatal("installed OMAP database did not re-encode byte-for-byte")
	}
}
