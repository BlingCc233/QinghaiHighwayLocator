package main

import "testing"

func TestParsePointFolder(t *testing.T) {
	category, subfolders, err := parsePointFolder(`行政许可\跨越公路/照片`, "行政许可")
	if err != nil {
		t.Fatal(err)
	}
	if category != "行政许可" || len(subfolders) != 2 || subfolders[0] != "跨越公路" || subfolders[1] != "照片" {
		t.Fatalf("category = %q, subfolders = %#v", category, subfolders)
	}
	category, subfolders, err = parsePointFolder("", "桥梁")
	if err != nil || category != "桥梁" || len(subfolders) != 0 {
		t.Fatalf("asset type fallback = %q, %#v, %v", category, subfolders, err)
	}
}

func TestParsePointFolderRejectsInvalidPaths(t *testing.T) {
	for _, folder := range []string{"", "/行政许可", "行政许可/../其他", `C:\\行政许可`, "行政许可//跨越", "行政许可/名称:错误"} {
		if _, _, err := parsePointFolder(folder, ""); err == nil {
			t.Errorf("accepted invalid folder %q", folder)
		}
	}
	if _, _, err := parsePointFolder("行政许可/跨越", "桥梁"); err == nil {
		t.Fatal("accepted mismatched asset type")
	}
}
