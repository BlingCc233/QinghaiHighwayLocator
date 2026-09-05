// Package omapnative implements the page codec used by the installed Windows
// build of OMAP. It is intentionally kept separate from object persistence:
// callers must validate a decoded database before any write is considered.
package omapnative

import (
	"crypto/md5"
	"fmt"
)

const (
	pageSize = 1024
	pageXor  = byte(0x8b)
)

// DecodeDatabase decodes an OMAP database byte-for-byte. OMAP applies its
// cipher independently to each 1024 byte page.
func DecodeDatabase(encrypted []byte, password string) ([]byte, error) {
	if len(encrypted) == 0 || len(encrypted)%pageSize != 0 {
		return nil, fmt.Errorf("OMAP database length is not a multiple of %d bytes", pageSize)
	}
	key := md5.Sum([]byte(password))
	plain := make([]byte, len(encrypted))
	for offset := 0; offset < len(encrypted); offset += pageSize {
		cryptPage(plain[offset:offset+pageSize], encrypted[offset:offset+pageSize], key[:], false)
	}
	return plain, nil
}

// EncodeDatabase is the inverse of DecodeDatabase.
func EncodeDatabase(plain []byte, password string) ([]byte, error) {
	if len(plain) == 0 || len(plain)%pageSize != 0 {
		return nil, fmt.Errorf("OMAP database length is not a multiple of %d bytes", pageSize)
	}
	key := md5.Sum([]byte(password))
	encrypted := make([]byte, len(plain))
	for offset := 0; offset < len(plain); offset += pageSize {
		cryptPage(encrypted[offset:offset+pageSize], plain[offset:offset+pageSize], key[:], true)
	}
	return encrypted, nil
}

func cryptPage(dst, src, key []byte, encrypt bool) {
	var state [8]byte
	var stream [8]byte
	var schedule [52]uint16
	expandIDEAKey(&schedule, key)

	for offset := 0; offset < len(src); offset++ {
		index := offset % len(stream)
		if index == 0 {
			stream = encryptIDEABlock(state, &schedule)
		}
		if encrypt {
			value := src[offset] ^ pageXor
			value ^= stream[index]
			dst[offset] = value
			state[index] = value
			continue
		}
		ciphertext := src[offset]
		dst[offset] = (ciphertext ^ stream[index]) ^ pageXor
		state[index] = ciphertext
	}
}

func expandIDEAKey(schedule *[52]uint16, key []byte) {
	for i := 0; i < 8; i++ {
		schedule[i] = uint16(key[i*2])<<8 | uint16(key[i*2+1])
	}
	for i := 8; i < len(schedule); i++ {
		switch i % 8 {
		case 6:
			schedule[i] = (schedule[i-7] << 9) | (schedule[i-14] >> 7)
		case 7:
			schedule[i] = (schedule[i-15] << 9) | (schedule[i-14] >> 7)
		default:
			schedule[i] = (schedule[i-7] << 9) | (schedule[i-6] >> 7)
		}
	}
}

func encryptIDEABlock(block [8]byte, schedule *[52]uint16) [8]byte {
	x1 := uint16(block[0])<<8 | uint16(block[1])
	x2 := uint16(block[2])<<8 | uint16(block[3])
	x3 := uint16(block[4])<<8 | uint16(block[5])
	x4 := uint16(block[6])<<8 | uint16(block[7])

	keyOffset := 0
	// The installed OMAP build deliberately uses four IDEA-style rounds. This
	// is not interoperable with standard eight-round IDEA implementations.
	for round := 0; round < 4; round++ {
		x1 = ideaMul(x1, schedule[keyOffset])
		x2 += schedule[keyOffset+1]
		x3 += schedule[keyOffset+2]
		x4 = ideaMul(x4, schedule[keyOffset+3])
		t0 := x1 ^ x3
		t1 := x2 ^ x4
		t0 = ideaMul(t0, schedule[keyOffset+4])
		t1 += t0
		t1 = ideaMul(t1, schedule[keyOffset+5])
		t0 += t1
		x1 ^= t1
		x4 ^= t0
		temp := x2
		x2 = x3 ^ t1
		x3 = temp ^ t0
		keyOffset += 6
	}

	x1 = ideaMul(x1, schedule[keyOffset])
	x3 += schedule[keyOffset+1]
	x2 += schedule[keyOffset+2]
	x4 = ideaMul(x4, schedule[keyOffset+3])

	return [8]byte{
		byte(x1 >> 8), byte(x1),
		byte(x3 >> 8), byte(x3),
		byte(x2 >> 8), byte(x2),
		byte(x4 >> 8), byte(x4),
	}
}

func ideaMul(a, b uint16) uint16 {
	if a == 0 {
		return uint16(1 - uint32(b))
	}
	if b == 0 {
		return uint16(1 - uint32(a))
	}
	product := uint32(a) * uint32(b)
	low := uint16(product)
	high := uint16(product >> 16)
	if low < high {
		return low - high + 1
	}
	return low - high
}
