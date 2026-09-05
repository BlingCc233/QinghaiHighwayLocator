package omapnative

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	_ "modernc.org/sqlite"
)

// TestInstalledObjectForensics only reads a configured backup. It records
// enough structure to reproduce OMAP's local object records without relying
// on UI import/export behavior.
func TestInstalledObjectForensics(t *testing.T) {
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
	copyPath := filepath.Join(t.TempDir(), "oobj.sqlite")
	if err := os.WriteFile(copyPath, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(copyPath)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	configRows, err := db.Query("SELECT name, valuestr FROM sec_config ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	defer configRows.Close()
	for configRows.Next() {
		var name, value string
		if err := configRows.Scan(&name, &value); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "config %s=%s\n", name, value)
	}

	rows, err := db.Query(`SELECT objid, parentid, srvid, type, modifytime, allchildcnt, md5d, md5g, md5s, data, groupdata
		FROM object WHERE type IN (7, 30) ORDER BY type DESC, objid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var objectID, parentID, serverID, objectType, modified, children int64
		var md5d, md5g, md5s sql.NullInt64
		var data, groupData []byte
		if err := rows.Scan(&objectID, &parentID, &serverID, &objectType, &modified, &children, &md5d, &md5g, &md5s, &data, &groupData); err != nil {
			t.Fatal(err)
		}
		if objectType == 30 {
			fmt.Fprintf(os.Stderr, "folder user=%d id=%d parent=%d server=%d name=%q children=%d group=%x hashes=(%d,%d,%d) datahash=%s grouphash=%s\n",
				0, objectID, parentID, serverID, folderName(data), children, groupData, md5d.Int64, md5g.Int64, md5s.Int64, hashSummary(data), hashSummary(groupData))
			if folderName(data) == "收藏夹" || folderName(data) == "西宁高速支队" || folderName(data) == "韵家口大队" || folderName(data) == "桥梁" || folderName(data) == "桩号" {
				fmt.Fprintf(os.Stderr, "folder-hash-candidates id=%d data=%s group=%s\n", objectID, hashCandidates(data), hashCandidates(groupData))
			}
			if folderName(data) == "收藏夹" || folderName(data) == "西宁高速支队" || folderName(data) == "韵家口大队" || folderName(data) == "桥梁" || folderName(data) == "桩号" {
				fmt.Fprintf(os.Stderr, "folder-hex id=%d data=%x\n", objectID, data)
				for offset := 0; offset+4 <= len(data); offset += 4 {
					value := binary.LittleEndian.Uint32(data[offset : offset+4])
					if value != 0 {
						fmt.Fprintf(os.Stderr, "folder-field id=%d offset=%d u32=%d hex=%x\n", objectID, offset, value, data[offset:offset+4])
					}
				}
			}
			continue
		}
		name := pointName(data)
		var attachmentIDs []uint64
		var attachmentOffsets []string
		for offset := 0; offset+8 <= min(len(data), 192); offset += 8 {
			value := binary.LittleEndian.Uint64(data[offset : offset+8])
			if value > 100000000000000000 {
				attachmentIDs = append(attachmentIDs, value)
				attachmentOffsets = append(attachmentOffsets, fmt.Sprintf("%d:%d", offset, value))
			}
		}
		fmt.Fprintf(os.Stderr, "point id=%d parent=%d server=%d data0=%d name=%q length=%d lat=%.8f lon=%.8f ext=%q ids=%v offsets=%v tail=%x hashes=(%d,%d,%d) datahash=%s\n",
			objectID, parentID, serverID, binary.LittleEndian.Uint64(data[:8]), name, len(data), pointLatitude(data), pointLongitude(data), fieldText(data, 56, 16), attachmentIDs,
			attachmentOffsets, data[max(0, len(data)-80):], md5d.Int64, md5g.Int64, md5s.Int64, hashSummary(data))
		if objectID == 562863864 || objectID == 141608614 || objectID == 24272277 {
			for offset := 0; offset < min(len(data), 336); offset += 16 {
				end := min(offset+16, len(data))
				fmt.Fprintf(os.Stderr, "point-hex id=%d offset=%03d %x\n", objectID, offset, data[offset:end])
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

// TestPointNameLayoutProbe verifies the variable-length name boundary against
// an object emitted by OMAP itself. It is deliberately read-only and opt-in.
func TestPointNameLayoutProbe(t *testing.T) {
	path := os.Getenv("OMAP_CODEC_PROBE")
	if path == "" {
		t.Skip("OMAP_CODEC_PROBE not set")
	}
	encrypted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecodeDatabase(encrypted, databasePassword)
	if err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(t.TempDir(), "oobj.sqlite")
	if err := os.WriteFile(databasePath, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var data []byte
	if err := database.QueryRow("SELECT data FROM object WHERE objid = ?", 24272277).Scan(&data); err != nil {
		t.Fatal(err)
	}
	needle := []byte("下旧庄黑林河大桥 K2323+927.0M")
	start := bytes.Index(data, needle)
	if start < 1 {
		t.Fatalf("known OMAP point name was not found, index=%d", start)
	}
	t.Logf("name starts at %d; preceding byte[%d]=%d; bytes[324:336]=% x; current constants length=%d name=%d parsed=%q", start, start-1, data[start-1], data[324:336], pointNameLengthOffset, pointNameOffset, pointName(data))
}

func hashCandidates(data []byte) string {
	if len(data) == 0 {
		return "empty"
	}
	var values []string
	for _, seed := range []uint32{0, 1, 0xffffffff, 0x811c9dc5, 0x9747b28c} {
		values = append(values, fmt.Sprintf("crc32c(%08x)=%d", seed, crc32.Update(seed, crc32.MakeTable(crc32.Castagnoli), data)))
	}
	values = append(values, fmt.Sprintf("adler=%d", adler32(data)), fmt.Sprintf("djb2=%d", djb2(data)), fmt.Sprintf("fnv1=%d", fnv1(data)), fmt.Sprintf("fnv1a=%d", fnv1a(data)), fmt.Sprintf("murmur=%d", murmur3(data, 0)), fmt.Sprintf("murmur1=%d", murmur3(data, 1)), fmt.Sprintf("hashl=%d", hashLittle(data, 0)), fmt.Sprintf("hashl1=%d", hashLittle(data, 1)))
	return strings.Join(values, " ")
}

func adler32(data []byte) uint32 {
	var a uint32 = 1
	var b uint32
	for _, value := range data {
		a = (a + uint32(value)) % 65521
		b = (b + a) % 65521
	}
	return b<<16 | a
}
func djb2(data []byte) uint32 {
	var value uint32 = 5381
	for _, byteValue := range data {
		value = value*33 + uint32(byteValue)
	}
	return value
}
func fnv1(data []byte) uint32 {
	var value uint32 = 2166136261
	for _, byteValue := range data {
		value *= 16777619
		value ^= uint32(byteValue)
	}
	return value
}
func fnv1a(data []byte) uint32 {
	var value uint32 = 2166136261
	for _, byteValue := range data {
		value ^= uint32(byteValue)
		value *= 16777619
	}
	return value
}
func murmur3(data []byte, seed uint32) uint32 {
	const c1 uint32 = 0xcc9e2d51
	const c2 uint32 = 0x1b873593
	hash := seed
	index := 0
	for ; index+4 <= len(data); index += 4 {
		value := binary.LittleEndian.Uint32(data[index:])
		value *= c1
		value = value<<15 | value>>17
		value *= c2
		hash ^= value
		hash = hash<<13 | hash>>19
		hash = hash*5 + 0xe6546b64
	}
	var tail uint32
	switch len(data) - index {
	case 3:
		tail ^= uint32(data[index+2]) << 16
		fallthrough
	case 2:
		tail ^= uint32(data[index+1]) << 8
		fallthrough
	case 1:
		tail ^= uint32(data[index])
		tail *= c1
		tail = tail<<15 | tail>>17
		tail *= c2
		hash ^= tail
	}
	hash ^= uint32(len(data))
	hash ^= hash >> 16
	hash *= 0x85ebca6b
	hash ^= hash >> 13
	hash *= 0xc2b2ae35
	return hash ^ hash>>16
}

func hashLittle(data []byte, init uint32) uint32 {
	hash := init
	length, index := len(data), 0
	for length >= 12 {
		a := hash + binary.LittleEndian.Uint32(data[index:])
		b := hash + binary.LittleEndian.Uint32(data[index+4:])
		c := hash + binary.LittleEndian.Uint32(data[index+8:])
		a -= c
		a ^= c<<4 | c>>28
		c += b
		b -= a
		b ^= a<<6 | a>>26
		a += c
		c -= b
		c ^= b<<8 | b>>24
		b += a
		a -= c
		a ^= c<<16 | c>>16
		c += b
		b -= a
		b ^= a<<19 | a>>13
		a += c
		c -= b
		c ^= b<<4 | b>>28
		b += a
		index += 12
		length -= 12
		hash = c
	}
	a, b, c := hash, hash, hash+uint32(length)
	for i := 0; i < length; i++ {
		shift := uint((i % 4) * 8)
		if i < 4 {
			a += uint32(data[index+i]) << shift
		} else if i < 8 {
			b += uint32(data[index+i]) << shift
		} else {
			c += uint32(data[index+i]) << shift
		}
	}
	c ^= b
	c -= b<<14 | b>>18
	a ^= c
	a -= c<<11 | c>>21
	b ^= a
	b -= a<<25 | a>>7
	c ^= b
	c -= b<<16 | b>>16
	a ^= c
	a -= c<<4 | c>>28
	b ^= a
	b -= a<<14 | a>>18
	c ^= b
	c -= b<<24 | b>>8
	return c
}

var _ = unsafe.Sizeof(0)

func folderName(data []byte) string {
	if len(data) < 102 {
		return ""
	}
	length := int(data[100])
	if 101+length > len(data) {
		return ""
	}
	return string(data[101 : 101+length])
}

func pointName(data []byte) string {
	if len(data) <= pointNameLengthOffset {
		return ""
	}
	length := int(data[pointNameLengthOffset])
	if pointNameOffset+length > len(data) {
		return ""
	}
	return string(data[pointNameOffset : pointNameOffset+length])
}

func pointLatitude(data []byte) float64 {
	if len(data) < 308 {
		return 0
	}
	return float64FromLittleEndian(data[300:308])
}

func pointLongitude(data []byte) float64 {
	if len(data) < 316 {
		return 0
	}
	return float64FromLittleEndian(data[308:316])
}

func float64FromLittleEndian(data []byte) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(data))
}

func fieldText(data []byte, offset, length int) string {
	if offset+length > len(data) {
		return ""
	}
	return strings.TrimRight(string(data[offset:offset+length]), "\x00")
}

func hashSummary(data []byte) string {
	md5Value := md5.Sum(data)
	sha1Value := sha1.Sum(data)
	return fmt.Sprintf("crc=%d md5=%x md5le=%d md5be=%d sha1le=%d", crc32.ChecksumIEEE(data), md5Value, binary.LittleEndian.Uint32(md5Value[:4]), binary.BigEndian.Uint32(md5Value[:4]), binary.LittleEndian.Uint32(sha1Value[:4]))
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
