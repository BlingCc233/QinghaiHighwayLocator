package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"stationnum2omap/internal/omapnative"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: inspect_omap_schema path-to-oobj.odb")
		os.Exit(2)
	}
	cipher, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	plain, err := omapnative.DecodeDatabase(cipher, "ovital318uinkme")
	if err != nil {
		panic(err)
	}
	dir, err := os.MkdirTemp("", "omap-schema-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	dbPath := filepath.Join(dir, "oobj.sqlite")
	if err := os.WriteFile(dbPath, plain, 0600); err != nil {
		panic(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type IN ('table','index') ORDER BY type, name")
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var ddl sql.NullString
		if err := rows.Scan(&name, &ddl); err != nil {
			panic(err)
		}
		fmt.Printf("%s\t%s\n", name, ddl.String)
	}
}
