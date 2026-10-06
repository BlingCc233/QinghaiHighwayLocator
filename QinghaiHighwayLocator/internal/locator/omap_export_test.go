package locator

import (
	"os"
	"path/filepath"
	"stationnum2omap/internal/omapnative"
	"strings"
	"testing"
)

func TestDefaultOmapDataDirectoryTargetsInstalledData(t *testing.T) {
	directory := filepath.Clean(DefaultOmapDataDirectory())
	if filepath.Base(directory) != "data" {
		t.Fatalf("OMAP target must be its data directory, got %q", directory)
	}
	if !strings.Contains(filepath.ToSlash(directory), "/omap/") {
		t.Fatalf("OMAP target must be under the OMAP profile, got %q", directory)
	}
}

func TestSyncOmapAttachmentsUsesObjectIDName(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "桥梁照片.jpg")
	if err := os.WriteFile(source, []byte("photo"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "omap-attachment")
	attachments := []OmapAttachment{{Name: filepath.Base(source), Path: source}}
	if err := syncOmapAttachments(attachments, 123456789, target); err != nil {
		t.Fatal(err)
	}
	if attachments[0].OmapName != "桥梁照片(123456789).jpg" {
		t.Fatalf("unexpected OMAP filename: %q", attachments[0].OmapName)
	}
	content, err := os.ReadFile(filepath.Join(target, attachments[0].OmapName))
	if err != nil || string(content) != "photo" {
		t.Fatalf("attachment was not copied: %v %q", err, content)
	}
}

func TestExportOmapValidatesAssetInput(t *testing.T) {
	_, err := ExportOmap(OmapPointInput{Station: "G6 K1792+200", AssetType: "不存在", Name: "点"})
	if err == nil {
		t.Fatal("expected asset type validation error")
	}
}

func TestNewAssetTypesMatchNativeFolders(t *testing.T) {
	for _, assetType := range []string{"涵洞", "涉路施工监管", "车辆通道"} {
		if _, ok := assetTypes[assetType]; !ok {
			t.Errorf("missing export asset type %q", assetType)
		}
	}
}

func TestExportOmapInputRetainsNativeContract(t *testing.T) {
	if omapnative.ErrNativeWriteDisabled == nil {
		t.Fatal("native compatibility sentinel must remain available")
	}
	if got := DefaultOmapAttachmentDirectory(); filepath.Base(got) != "attachment" {
		t.Fatalf("unexpected OMAP attachment directory: %q", got)
	}
}

func TestParseRangeKilometre(t *testing.T) {
	for _, test := range []struct {
		input string
		want  int
	}{
		{input: "1766", want: 1766},
		{input: "K1767", want: 1767},
		{input: " k1836 ", want: 1836},
	} {
		got, err := parseRangeKilometre(test.input)
		if err != nil || got != test.want {
			t.Fatalf("parseRangeKilometre(%q) = %d, %v; want %d", test.input, got, err, test.want)
		}
	}
	for _, input := range []string{"", "K1766+500", "17a6", "-1"} {
		if _, err := parseRangeKilometre(input); err == nil {
			t.Fatalf("parseRangeKilometre(%q) should reject non-integer kilometre", input)
		}
	}
}

func TestObjectNameAppendsRouteAndStation(t *testing.T) {
	result := Result{Route: "G6", Station: "K1792+200"}
	if got := objectName("海东收费站", result, "收费站"); got != "海东收费站 G6 K1792+200" {
		t.Fatalf("unexpected object name: %q", got)
	}
	if got := objectName("海东收费站 G6 K1792+200", result, "收费站"); got != "海东收费站 G6 K1792+200" {
		t.Fatalf("object name should not duplicate station: %q", got)
	}
}

func TestObjectNameForStationOnly(t *testing.T) {
	result := Result{Route: "G6", Station: "K1792+000"}
	if got := objectName("ignored", result, "桩号"); got != "G6 K1792+000" {
		t.Fatalf("桩号对象名必须只包含完整线路桩号: %q", got)
	}
}
