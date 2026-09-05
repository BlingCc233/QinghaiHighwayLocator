package omapnative

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestInspectInstalledBackup is an opt-in, read-only schema probe. It writes
// only a decoded copy into Go's temporary test directory.
func TestInspectInstalledBackup(t *testing.T) {
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
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(copyPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatal(err)
	}
	if integrity != "ok" {
		t.Fatalf("decoded database integrity check: %s", integrity)
	}
	rows, err := db.Query("SELECT type, name, sql FROM sqlite_master WHERE type IN ('table', 'index') ORDER BY type, name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind, name string
		var definition sql.NullString
		if err := rows.Scan(&kind, &name, &definition); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "%s %s %s\n", kind, name, definition.String)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	objectRows, err := db.Query(`SELECT userid, objid, parentid, srvid, type, modifytime, allchildcnt,
		md5d, md5g, md5s, length(data), length(groupdata) FROM object ORDER BY objid LIMIT 200`)
	if err != nil {
		t.Fatal(err)
	}
	defer objectRows.Close()
	for objectRows.Next() {
		var userID, objectID, parentID, serverID, objectType, modifyTime, childCount sql.NullInt64
		var md5Data, md5Group, md5Style sql.NullString
		var dataLength, groupLength sql.NullInt64
		if err := objectRows.Scan(&userID, &objectID, &parentID, &serverID, &objectType, &modifyTime, &childCount,
			&md5Data, &md5Group, &md5Style, &dataLength, &groupLength); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "object user=%d id=%d parent=%d server=%d type=%d modified=%d children=%d md5=(%s,%s,%s) data=%d group=%d\n",
			userID.Int64, objectID.Int64, parentID.Int64, serverID.Int64, objectType.Int64, modifyTime.Int64, childCount.Int64,
			md5Data.String, md5Group.String, md5Style.String, dataLength.Int64, groupLength.Int64)
	}
	if err := objectRows.Err(); err != nil {
		t.Fatal(err)
	}

	payloadRows, err := db.Query(`SELECT objid, parentid, type, data, groupdata FROM object
		WHERE type IN (7, 20, 30) ORDER BY type DESC, objid`)
	if err != nil {
		t.Fatal(err)
	}
	defer payloadRows.Close()
	for payloadRows.Next() {
		var objectID, parentID, objectType int64
		var data, groupData []byte
		if err := payloadRows.Scan(&objectID, &parentID, &objectType, &data, &groupData); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "payload id=%d parent=%d type=%d data=%s group=%s\n",
			objectID, parentID, objectType, escapedPayload(data), escapedPayload(groupData))
	}
	if err := payloadRows.Err(); err != nil {
		t.Fatal(err)
	}

	pointRows, err := db.Query(`SELECT objid, parentid, data FROM object WHERE type = 7 ORDER BY objid`)
	if err != nil {
		t.Fatal(err)
	}
	defer pointRows.Close()
	for pointRows.Next() {
		var objectID, parentID int64
		var data []byte
		if err := pointRows.Scan(&objectID, &parentID, &data); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "point-summary id=%d parent=%d strings=%q doubles=%s\n",
			objectID, parentID, printableStrings(data), coordinateCandidates(data))
	}
	if err := pointRows.Err(); err != nil {
		t.Fatal(err)
	}
}

func escapedPayload(data []byte) string {
	if len(data) == 0 {
		return "<empty>"
	}
	var builder strings.Builder
	for _, value := range data {
		if value >= 0x20 && value <= 0x7e && value != '\\' {
			builder.WriteByte(value)
		} else {
			builder.WriteString("\\x")
			builder.WriteString(strings.ToUpper(strconv.FormatUint(uint64(value), 16)))
		}
	}
	return builder.String()
}

func printableStrings(data []byte) []string {
	var values []string
	for offset := 0; offset < len(data); {
		end := bytes.IndexByte(data[offset:], 0)
		if end < 0 {
			break
		}
		end += offset
		value := string(data[offset:end])
		if len(value) >= 3 && isUTF8Text(value) {
			values = append(values, fmt.Sprintf("%d:%s", offset, value))
		}
		offset = end + 1
	}
	return values
}

func isUTF8Text(value string) bool {
	for _, runeValue := range value {
		if runeValue < 0x20 {
			return false
		}
	}
	return true
}

func coordinateCandidates(data []byte) string {
	var values []string
	for offset := 0; offset+8 <= len(data); offset += 1 {
		value := math.Float64frombits(binary.LittleEndian.Uint64(data[offset : offset+8]))
		if (value >= 20 && value <= 55) || (value >= 70 && value <= 140) {
			values = append(values, fmt.Sprintf("%d=%.8f", offset, value))
		}
	}
	return strings.Join(values, ",")
}
