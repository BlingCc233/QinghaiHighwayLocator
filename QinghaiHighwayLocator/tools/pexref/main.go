package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	imageBase = uint64(0x140000000)
	textRaw   = uint64(0x400)
	textRVA   = uint64(0x1000)
	textSize  = uint64(88155136)
	pdataRaw  = uint64(0x8ebc400)
	pdataSize = uint64(3065856)
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: pexref <omap.exe> <target-va-hex>")
		os.Exit(2)
	}
	var target uint64
	if _, err := fmt.Sscanf(os.Args[2], "%x", &target); err != nil {
		panic(err)
	}
	image, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	text := image[textRaw : textRaw+textSize]
	for offset := 0; offset+7 <= len(text); offset++ {
		if !isRIPRelative(text[offset : offset+3]) {
			continue
		}
		displacement := int64(int32(binary.LittleEndian.Uint32(text[offset+3 : offset+7])))
		address := int64(imageBase+textRVA+uint64(offset)+7) + displacement
		if uint64(address) != target {
			continue
		}
		start, end := offset-16, offset+32
		if start < 0 {
			start = 0
		}
		if end > len(text) {
			end = len(text)
		}
		xref := imageBase + textRVA + uint64(offset)
		functionStart, functionEnd := functionBounds(image, uint32(xref-imageBase))
		fmt.Printf("xref VA=%#x raw=%#x function=%#x-%#x bytes=%x\\n", xref, textRaw+uint64(offset), imageBase+uint64(functionStart), imageBase+uint64(functionEnd), text[start:end])
	}
	for offset := 0; offset+5 <= len(text); offset++ {
		if text[offset] != 0xe8 {
			continue
		}
		displacement := int64(int32(binary.LittleEndian.Uint32(text[offset+1 : offset+5])))
		address := int64(imageBase+textRVA+uint64(offset)+5) + displacement
		if uint64(address) != target {
			continue
		}
		call := imageBase + textRVA + uint64(offset)
		functionStart, functionEnd := functionBounds(image, uint32(call-imageBase))
		fmt.Printf("call VA=%#x raw=%#x function=%#x-%#x\\n", call, textRaw+uint64(offset), imageBase+uint64(functionStart), imageBase+uint64(functionEnd))
	}
}

func functionBounds(image []byte, target uint32) (uint32, uint32) {
	pdata := image[pdataRaw : pdataRaw+pdataSize]
	for offset := 0; offset+12 <= len(pdata); offset += 12 {
		start := binary.LittleEndian.Uint32(pdata[offset:])
		end := binary.LittleEndian.Uint32(pdata[offset+4:])
		if start <= target && target < end {
			return start, end
		}
	}
	return 0, 0
}

func isRIPRelative(instruction []byte) bool {
	if instruction[0] < 0x40 || instruction[0] > 0x4f {
		return false
	}
	if instruction[1] != 0x8d && instruction[1] != 0x8b && instruction[1] != 0x89 {
		return false
	}
	return instruction[2]&0xc7 == 0x05
}
