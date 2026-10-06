package main

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"stationnum2omap/internal/omapnative"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: dump_omap_points path-to-oobj.odb")
	}
	cipher, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	plain, err := omapnative.DecodeDatabase(cipher, "ovital318uinkme")
	if err != nil {
		panic(err)
	}
	dir, err := os.MkdirTemp("", "omap-dump-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "oobj.sqlite")
	if err := os.WriteFile(path, plain, 0600); err != nil {
		panic(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT objid,parentid,data FROM object WHERE type=7 ORDER BY objid")
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, parent int64
		var data []byte
		if err := rows.Scan(&id, &parent, &data); err != nil {
			panic(err)
		}
		name := ""
		nameLen := 0
		if len(data) > 332 {
			nameLen = int(data[331])
			if nameLen > 0 && 332+nameLen <= len(data) {
				name = string(data[332 : 332+nameLen])
			}
		}
		fmt.Printf("id=%d parent=%d len=%d nameBytes=%d extra=%d name=%q\n", id, parent, len(data), nameLen, len(data)-(332+nameLen+6), name)
		end := 332 + nameLen + 6
		if end < len(data) {
			fmt.Printf("extraHex=%s\n", hex.EncodeToString(data[end:]))
		}
		fmt.Printf("hex0=%s\n", hex.EncodeToString(data[:min(len(data), 420)]))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
