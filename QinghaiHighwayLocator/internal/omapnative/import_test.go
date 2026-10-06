package omapnative

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
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
	attachmentOne := filepath.Join(root, "现场照片.jpg")
	if err := os.WriteFile(attachmentOne, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Import(ImportInput{DataDirectory: dataDirectory, AssetType: "桩号", Subfolders: []string{"测试子目录"}, Name: "兼容性探针 G6 K1792+200", Latitude: 36.5458288, Longitude: 101.9648821, Comment: "测试备注", Attachments: []string{attachmentOne}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ObjectID == 0 {
		t.Fatal("writer returned an empty object ID")
	}
	if len(result.Attachments) != 1 || filepath.Ext(result.Attachments[0].OmapName) != ".jpg" {
		t.Fatalf("single image attachment = %#v", result.Attachments)
	}
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
	nameEnd := pointNameOffset + int(contents[pointNameLengthOffset]) + pointSuffixLength
	if !strings.Contains(string(contents[nameEnd:]), "测试备注") {
		t.Fatalf("written point comment missing: %x", contents[nameEnd:])
	}
	if got := contents[272:276]; !bytes.Equal(got, []byte{4, 0, 1, 0}) {
		t.Fatalf("written point editable metadata = %x", got)
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
	if folderName(categoryData) != "测试子目录" {
		t.Fatalf("point parent = %q, want 测试子目录", folderName(categoryData))
	}
	var categoryParentID int64
	if err := db.QueryRow("SELECT parentid FROM object WHERE objid = ?", parentID).Scan(&categoryParentID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT data FROM object WHERE objid = ?", categoryParentID).Scan(&categoryData); err != nil {
		t.Fatal(err)
	}
	if folderName(categoryData) != "桩号" {
		t.Fatalf("subfolder parent = %q, want 桩号", folderName(categoryData))
	}
	if !strings.HasSuffix(result.TargetFolder, "桩号 > 测试子目录") {
		t.Fatalf("target folder = %q", result.TargetFolder)
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
	binary.LittleEndian.PutUint32(template[pointAltitudeOffset:pointAltitudeOffset+4], 3500)
	for index := pointAttachmentOffset; index < pointExtensionOffset+16; index++ {
		template[index] = 0xff
	}

	altitude := int32(2200)
	data, err := makePointData(template, "韵家口收费站", 36.5458288, 101.9648821, &altitude, "", nil, 1788600000)
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
	if got := binary.LittleEndian.Uint32(data[pointAltitudeOffset : pointAltitudeOffset+4]); got != 2200 {
		t.Fatalf("new point altitude = %d, want 2200", got)
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

func TestMakePointDataNormalizesEditableMetadata(t *testing.T) {
	template := make([]byte, 384)
	copy(template[272:276], []byte{0, 0, 0, 0})
	data, err := makePointData(template, "可拖动点", 36, 101, nil, "", nil, 1788600000)
	if err != nil {
		t.Fatal(err)
	}
	if got := data[272:276]; !bytes.Equal(got, []byte{4, 0, 1, 0}) {
		t.Fatalf("editable metadata = %x, want 04000100", got)
	}
}

func TestMakePointDataWithoutAltitudePreservesTemplate(t *testing.T) {
	template := make([]byte, 384)
	binary.LittleEndian.PutUint32(template[pointAltitudeOffset:pointAltitudeOffset+4], 3500)
	data, err := makePointData(template, "测试", 36, 101, nil, "", nil, 1788600000)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(data[pointAltitudeOffset : pointAltitudeOffset+4]); got != 3500 {
		t.Fatalf("template altitude = %d, want 3500", got)
	}
}

func TestMakePointDataRejectsMultipleAttachments(t *testing.T) {
	template := make([]byte, 384)
	_, err := makePointData(template, "测试", 36, 101, nil, "", []preparedAttachment{{id: 1}, {id: 2}}, 1788600000)
	if err == nil {
		t.Fatal("expected multi-attachment error")
	}
}

func TestNewAssetCategories(t *testing.T) {
	for _, name := range []string{"涵洞", "涉路施工监管", "车辆通道"} {
		if got, ok := categoryName(name); !ok || got != name {
			t.Errorf("categoryName(%q) = %q, %t", name, got, ok)
		}
	}
}

func TestValidateSubfolders(t *testing.T) {
	if err := validateSubfolders([]string{"跨越公路", "许可照片"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", ".", "..", "../其他大队", `C:\\data`, "名称/目录", "名称\n目录"} {
		if err := validateSubfolders([]string{name}); err == nil {
			t.Errorf("accepted invalid subfolder %q", name)
		}
	}
}

func TestPrepareAttachmentsRejectsNonImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "说明.docx")
	if err := os.WriteFile(path, []byte("doc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareAttachments([]string{path}); err == nil {
		t.Fatal("expected non-image rejection")
	}
}

func TestMakePointDataWritesComment(t *testing.T) {
	template := make([]byte, 384)
	data, err := makePointData(template, "测试点", 36, 101, nil, "许可编号：青交许字〔2026〕1号\n备注", nil, 1788600000)
	if err != nil {
		t.Fatal(err)
	}
	nameLen := int(data[pointNameLengthOffset])
	comment := data[pointNameOffset+nameLen+pointSuffixLength:]
	if string(comment[:len(comment)-1]) != "许可编号：青交许字〔2026〕1号\n备注" || comment[len(comment)-1] != 0 {
		t.Fatalf("comment bytes = %x", comment)
	}
}
