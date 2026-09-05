package omapnative

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSchemaProbe(t *testing.T) {
	path := os.Getenv("OMAP_CODEC_PROBE")
	if path == "" {
		t.Skip("OMAP_CODEC_PROBE not set")
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecodeDatabase(encoded, databasePassword)
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "probe.sqlite")
	if err := os.WriteFile(dbPath, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT type, name, tbl_name, sql FROM sqlite_master ORDER BY type, name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var typ, name, table string
		var statement sql.NullString
		if err := rows.Scan(&typ, &name, &table, &statement); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "%s %s %s %s\\n", typ, name, table, statement.String)
	}
}

func TestObjectTreeProbe(t *testing.T) {
	path := os.Getenv("OMAP_CODEC_PROBE")
	if path == "" {
		t.Skip("OMAP_CODEC_PROBE not set")
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecodeDatabase(encoded, databasePassword)
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "probe.sqlite")
	if err := os.WriteFile(dbPath, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT objid, parentid, type, allchildcnt, length(data), length(groupdata), data FROM object ORDER BY parentid, objid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, parent, typ, childCount, dataLength int64
		var groupLength sql.NullInt64
		var data []byte
		if err := rows.Scan(&id, &parent, &typ, &childCount, &dataLength, &groupLength, &data); err != nil {
			t.Fatal(err)
		}
		name := ""
		if typ == folderObjectType {
			name = folderName(data)
		} else if typ == pointObjectType {
			name = pointName(data)
		}
		fmt.Fprintf(os.Stderr, "object id=%d parent=%d type=%d name=%q children=%d data=%d group=%d\\n", id, parent, typ, name, childCount, dataLength, groupLength.Int64)
	}
}
