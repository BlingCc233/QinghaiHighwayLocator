package omapnative

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestFolderStateHashProbe compares likely recursive hash inputs with OMAP's
// recorded md5s. It is opt-in and only reads a historical OMAP snapshot.
func TestFolderStateHashProbe(t *testing.T) {
	path := os.Getenv("OMAP_STATE_PROBE")
	if path == "" {
		t.Skip("OMAP_STATE_PROBE not set")
	}
	objects := loadHistoricalObjects(t, path)
	for _, folderID := range []int64{539362937, 1541200944, 1712247025, 324510663, 1123692839} {
		folder, ok := objects[folderID]
		if !ok {
			continue
		}
		children := make([]historicalObject, 0, len(folder.groupData)/4)
		for offset := 0; offset+4 <= len(folder.groupData); offset += 4 {
			id := int64(binary.LittleEndian.Uint32(folder.groupData[offset : offset+4]))
			if child, exists := objects[id]; exists {
				children = append(children, child)
			}
		}
		bufData := append([]byte(nil), folder.groupData...)
		bufHashes := make([]byte, 0, len(children)*12)
		for _, child := range children {
			var item [12]byte
			binary.LittleEndian.PutUint32(item[0:4], uint32(child.id))
			binary.LittleEndian.PutUint32(item[4:8], uint32(child.dataHash))
			binary.LittleEndian.PutUint32(item[8:12], uint32(child.groupHash))
			bufHashes = append(bufHashes, item[:]...)
		}
		fmt.Fprintf(os.Stderr, "STATE folder=%d name=%q md5s=%d group=%x candidates group=%d hashes=%d data+group=%d group+hashes=%d\n",
			folder.id, folderName(folder.data), folder.stateHash, folder.groupData,
			hashLittle(folder.groupData, 0), hashLittle(bufHashes, 0), objectHash(append(append([]byte(nil), folder.data...), folder.groupData...)), hashLittle(append(bufData, bufHashes...), 0))
	}
}

type historicalObject struct {
	id         int64
	parentID   int64
	serverID   int64
	objectType int64
	modifiedAt int64
	childCount int64
	dataHash   int64
	groupHash  int64
	stateHash  int64
	data       []byte
	groupData  []byte
}

// TestOmapHistoryProbe reads OMAP-created snapshot files only. Set
// OMAP_HISTORY_PROBE to the installed sys_backup directory to compare every
// recorded version and reveal how the client mutates object-tree records.
func TestOmapHistoryProbe(t *testing.T) {
	directory := os.Getenv("OMAP_HISTORY_PROBE")
	if directory == "" {
		t.Skip("OMAP_HISTORY_PROBE not set")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "oobj.odb") {
			continue
		}
		paths = append(paths, filepath.Join(directory, entry.Name()))
	}
	sort.Slice(paths, func(i, j int) bool {
		left, _ := os.Stat(paths[i])
		right, _ := os.Stat(paths[j])
		return left.ModTime().Before(right.ModTime())
	})
	if len(paths) < 2 {
		t.Fatalf("expected at least two history files, found %d", len(paths))
	}

	var previous map[int64]historicalObject
	var previousName string
	for _, path := range paths {
		objects := loadHistoricalObjects(t, path)
		fmt.Fprintf(os.Stderr, "SNAPSHOT %s objects=%d\n", filepath.Base(path), len(objects))
		if previous != nil {
			reportHistoricalDelta(previousName, previous, filepath.Base(path), objects)
		}
		previous = objects
		previousName = filepath.Base(path)
	}
}

func loadHistoricalObjects(t *testing.T, path string) map[int64]historicalObject {
	t.Helper()
	encrypted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecodeDatabase(encrypted, databasePassword)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	databasePath := filepath.Join(t.TempDir(), "history.sqlite")
	if err := os.WriteFile(databasePath, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	rows, err := database.Query(`SELECT objid, parentid, srvid, type, modifytime, allchildcnt,
		md5d, md5g, md5s, data, groupdata FROM object ORDER BY objid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	objects := make(map[int64]historicalObject)
	for rows.Next() {
		var object historicalObject
		var dataHash, groupHash, stateHash sql.NullInt64
		if err := rows.Scan(&object.id, &object.parentID, &object.serverID, &object.objectType,
			&object.modifiedAt, &object.childCount, &dataHash, &groupHash, &stateHash,
			&object.data, &object.groupData); err != nil {
			t.Fatal(err)
		}
		object.dataHash = dataHash.Int64
		object.groupHash = groupHash.Int64
		object.stateHash = stateHash.Int64
		objects[object.id] = object
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return objects
}

func reportHistoricalDelta(previousName string, previous map[int64]historicalObject, currentName string, current map[int64]historicalObject) {
	added, changed, removed := 0, 0, 0
	for id, after := range current {
		before, existed := previous[id]
		if !existed {
			added++
			if isRelevantHistoricalObject(after) {
				fmt.Fprintf(os.Stderr, "  ADD %s id=%d %s\n", currentName, id, historicalSummary(after))
			}
			continue
		}
		if !sameHistoricalObject(before, after) {
			changed++
			if isRelevantHistoricalObject(before) || isRelevantHistoricalObject(after) {
				fmt.Fprintf(os.Stderr, "  CHANGE %s -> %s id=%d\n    before %s\n    after  %s\n", previousName, currentName, id, historicalSummary(before), historicalSummary(after))
			}
		}
	}
	for id := range previous {
		if _, exists := current[id]; !exists {
			removed++
		}
	}
	fmt.Fprintf(os.Stderr, "DELTA %s -> %s added=%d changed=%d removed=%d\n", previousName, currentName, added, changed, removed)
}

func isRelevantHistoricalObject(object historicalObject) bool {
	return object.objectType == folderObjectType || object.objectType == pointObjectType
}

func sameHistoricalObject(left, right historicalObject) bool {
	return left.parentID == right.parentID && left.serverID == right.serverID && left.objectType == right.objectType &&
		left.modifiedAt == right.modifiedAt && left.childCount == right.childCount && left.dataHash == right.dataHash &&
		left.groupHash == right.groupHash && left.stateHash == right.stateHash &&
		string(left.data) == string(right.data) && string(left.groupData) == string(right.groupData)
}

func historicalSummary(object historicalObject) string {
	name := ""
	if object.objectType == folderObjectType {
		name = folderObjectName(object.data)
	} else if object.objectType == pointObjectType {
		name = pointName(object.data)
	}
	return fmt.Sprintf("type=%d name=%q parent=%d server=%d time=%d children=%d hashes=(d:%d g:%d s:%d) data=%s group=%x",
		object.objectType, name, object.parentID, object.serverID, object.modifiedAt, object.childCount,
		object.dataHash, object.groupHash, object.stateHash, shortHash(object.data), object.groupData)
}

func shortHash(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x/%d", sum[:6], len(data))
}
