package omapnative

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestImportOnCopy exercises the native writer against a disposable copy of
// an OMAP-created database. It never targets the installed profile.
func TestImportOnCopy(t *testing.T) {
	source := os.Getenv("OMAP_IMPORT_PROBE")
	if source == "" {
		t.Skip("OMAP_IMPORT_PROBE not set")
	}
	root := t.TempDir()
	dataDirectory := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDirectory, databaseFileName), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	attachmentOne := filepath.Join(root, "巡查记录.txt")
	attachmentTwo := filepath.Join(root, "现场照片.jpg")
	if err := os.WriteFile(attachmentOne, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attachmentTwo, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Import(ImportInput{DataDirectory: dataDirectory, AssetType: "桩号", Name: "兼容性探针 G6 K1792+200", Latitude: 36.5458288, Longitude: 101.9648821, Attachments: []string{attachmentOne, attachmentTwo}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ObjectID == 0 {
		t.Fatal("writer returned an empty object ID")
	}
	if len(result.Attachments) != 1 || filepath.Ext(result.Attachments[0].OmapName) != ".zip" {
		t.Fatalf("multi-attachment import should create one ZIP attachment, got %#v", result.Attachments)
	}
	archive, err := zip.OpenReader(filepath.Join(dataDirectory, "attachment", result.Attachments[0].OmapName))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 2 {
		archive.Close()
		t.Fatalf("installed attachment ZIP entries = %d, want 2", len(archive.File))
	}
	archive.Close()
	encoded, err := os.ReadFile(filepath.Join(dataDirectory, databaseFileName))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecodeDatabase(encoded, databasePassword)
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(root, "result.sqlite")
	if err := os.WriteFile(dbPath, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name string
	if err := db.QueryRow("SELECT data FROM object WHERE objid = ?", result.ObjectID).Scan(&contents); err != nil {
		t.Fatal(err)
	}
	name = pointName(contents)
	if name != "兼容性探针 G6 K1792+200" {
		t.Fatalf("written point name = %q", name)
	}
	var state sql.NullInt64
	if err := db.QueryRow("SELECT md5s FROM object WHERE objid = ?", result.ObjectID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state.Int64 != 0 {
		t.Fatalf("new point md5s = %d, want 0", state.Int64)
	}
	var parentID int64
	if err := db.QueryRow("SELECT parentid FROM object WHERE objid = ?", result.ObjectID).Scan(&parentID); err != nil {
		t.Fatal(err)
	}
	var categoryData []byte
	if err := db.QueryRow("SELECT data FROM object WHERE objid = ?", parentID).Scan(&categoryData); err != nil {
		t.Fatal(err)
	}
	if folderName(categoryData) != "桩号" {
		t.Fatalf("point parent = %q, want 桩号", folderName(categoryData))
	}
	var groupData []byte
	if err := db.QueryRow("SELECT groupdata FROM object WHERE objid = ?", parentID).Scan(&groupData); err != nil {
		t.Fatal(err)
	}
	if len(groupData) < 4 || binary.LittleEndian.Uint32(groupData[:4]) != uint32(result.ObjectID) {
		t.Fatalf("new point was not prepended to parent groupdata: %x", groupData)
	}
}

func TestMakePointDataPreservesFixedMetadata(t *testing.T) {
	template := make([]byte, 384)
	copy(template[272:276], []byte{4, 0, 1, 0})
	for index := pointAttachmentOffset; index < pointExtensionOffset+16; index++ {
		template[index] = 0xff
	}

	data, err := makePointData(template, "韵家口收费站", 36.5458288, 101.9648821, nil, 1788600000)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(data), pointNameOffset+len([]byte("韵家口收费站"))+pointSuffixLength; got != want {
		t.Fatalf("point length = %d, want %d", got, want)
	}
	if got := pointName(data); got != "韵家口收费站" {
		t.Fatalf("point name = %q", got)
	}
	if got := math.Float64frombits(binary.LittleEndian.Uint64(data[pointLatitudeOffset : pointLatitudeOffset+8])); got != 36.5458288 {
		t.Fatalf("latitude = %.7f", got)
	}
	if got := math.Float64frombits(binary.LittleEndian.Uint64(data[pointLongitudeOffset : pointLongitudeOffset+8])); got != 101.9648821 {
		t.Fatalf("longitude = %.7f", got)
	}
	if got := data[272:276]; !bytes.Equal(got, []byte{4, 0, 1, 0}) {
		t.Fatalf("fixed metadata was overwritten: %x", got)
	}
	if got := data[pointAttachmentOffset : pointExtensionOffset+16]; !bytes.Equal(got, make([]byte, len(got))) {
		t.Fatalf("attachment metadata was not cleared: %x", got)
	}
	if got := data[len(data)-pointSuffixLength:]; !bytes.Equal(got, make([]byte, pointSuffixLength)) {
		t.Fatalf("point suffix = %x", got)
	}
}

func TestMakePointDataRejectsMultipleAttachments(t *testing.T) {
	template := make([]byte, 384)
	_, err := makePointData(template, "测试", 36, 101, []preparedAttachment{{id: 1}, {id: 2}}, 1788600000)
	if err == nil {
		t.Fatal("expected multi-attachment error")
	}
}

func TestCreateAttachmentBundlePreservesFiles(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "巡查记录.txt")
	second := filepath.Join(directory, "现场照片.txt")
	if err := os.WriteFile(first, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	bundle, err := createAttachmentBundle([]preparedAttachment{{sourcePath: first}, {sourcePath: second}})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(bundle)
	archive, err := zip.OpenReader(bundle)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 2 {
		t.Fatalf("bundle entries = %d, want 2", len(archive.File))
	}
	for _, entry := range archive.File {
		reader, openErr := entry.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		contents, readErr := io.ReadAll(reader)
		reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(contents) != "one" && string(contents) != "two" {
			t.Fatalf("unexpected bundle content %q", contents)
		}
	}
}
