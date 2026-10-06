package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"stationnum2omap/internal/locator"
)

// The CLI deliberately keeps its input contract filesystem based. An agent can
// drop files into a road-property directory and invoke `import`; a JSON
// manifest is optional and is useful when a filename does not contain a station.
const stateName = ".qinghai-omap-import-state.json"

var version = "0.0.2"

var stationRE = regexp.MustCompile(`(?i)(?:(G0611|G0612|G341|G569|G6|S101|S104)\s*[KＫ]?\s*(\d{1,4})(?:\s*\+\s*(\d{1,3}))?|[KＫ]\s*(\d{1,4})(?:\s*\+\s*(\d{1,3}))?)`)

type item struct {
	Path        string   `json:"path"`
	Station     string   `json:"station"`
	SegmentID   string   `json:"segmentId"`
	Brigade     string   `json:"brigade"`
	AssetType   string   `json:"assetType"`
	Name        string   `json:"name"`
	Comment     string   `json:"comment,omitempty"`
	Attachments []string `json:"attachments"`
	Fingerprint string   `json:"fingerprint"`
}

type state struct {
	Version int                  `json:"version"`
	Sources map[string]string    `json:"sources"`
	Omap    map[string]fileState `json:"omap"`
	Updated string               `json:"updated"`
}

type fileState struct {
	Size    int64 `json:"size"`
	ModTime int64 `json:"mtime"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "--version", "-version":
		fmt.Printf("omap-import %s\n", version)
	case "scan":
		runScan(os.Args[2:], false)
	case "import":
		runScan(os.Args[2:], true)
	case "point":
		runPoint(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "delta":
		runDelta(os.Args[2:])
	case "help", "-h", "--help":
		if len(os.Args) > 2 && os.Args[1] == "help" {
			showCommandHelp(os.Args[2])
		} else {
			usage()
		}
	default:
		fmt.Fprintf(os.Stderr, "未知命令 %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`omap-import - 韵家口大队路产档案增量导入工具

用法：omap-import <命令> [选项]

命令：
  point   单独录入一个点、一张图片及文字备注
  scan    扫描档案目录并预览识别结果
  import  只导入档案目录中新增或变更的图片
  status  查看档案和 OMap data 的变更状态
  delta   复制 OMap data 中新增或变更的文件

帮助：omap-import help point；所有命令也支持 -h 和 --help。
OMap data 默认取 QINGHAI_OMAP_DATA，其次 OMAP_DATA_DIR；point 和 delta 可用 --data 覆盖。`)
}

func showCommandHelp(command string) {
	switch command {
	case "point":
		runPoint([]string{"-h"})
	case "scan":
		runScan([]string{"-h"}, false)
	case "import":
		runScan([]string{"-h"}, true)
	case "status":
		runStatus([]string{"-h"})
	case "delta":
		runDelta([]string{"-h"})
	default:
		fail("未知命令：" + command)
	}
}

func commandFlags(name, description, example string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "用法：omap-import %s [选项]\n\n%s\n\n示例：\n  %s\n\n选项：\n", name, description, example)
		fs.PrintDefaults()
	}
	return fs
}

func runPoint(args []string) {
	fs := commandFlags("point", "按所选路段把桩号换算为经纬度，写入可编辑的 OMap 点。\n--folder 是韵家口大队下的相对目录；首级必须是路产类型，如 行政许可/跨越公路。\n韵家口路段：xjk-g6-pingxi (G6 K1766+600-K1800+500)、xjk-g6-xiguojing (G6 K1800+500-K1836+000)、xjk-s101 (S101 K0+000-K41+430)。\n常用类型：行政许可、桥梁、涵洞、隧道、收费站、服务区、停车区、涉路施工、监控设施、安全隐患、桩号。\n每个点必须且只能附一张图片；运行前请完全退出 OMap。", "omap-import point --segment xjk-g6-xiguojing --station 'G6 K1807+228' --folder '行政许可/跨越公路' --name '跨越公路架设电缆' --comment '许可编号：...' --attachment 'C:\\档案\\57+060.jpg' --data 'C:\\Users\\13421\\Documents\\omap\\data' --json")
	segment := fs.String("segment", "", "必填；路段 ID，例如 xjk-g6-xiguojing")
	station := fs.String("station", "", "必填；完整桩号，例如 G6 K1807+228")
	brigade := fs.String("brigade", "韵家口大队", "仅支持韵家口大队")
	assetType := fs.String("asset-type", "", "路产类型；未给 --folder 时必填，例如 桥梁")
	folder := fs.String("folder", "", "大队下的相对目录，首级为路产类型，例如 行政许可/跨越公路")
	name := fs.String("name", "", "路产点必填；保存时自动追加标准桩号")
	comment := fs.String("comment", "", "点位自身的文字备注")
	attachments := multiFlag{}
	fs.Var(&attachments, "attachment", "必填；单张图片路径，不接受 ZIP、DOC 等文件")
	data := fs.String("data", "", "OMap data 目录；优先于环境变量")
	jsonOut := fs.Bool("json", false, "输出 JSON 结果")
	_ = fs.Parse(args)
	dataDir := strings.TrimSpace(*data)
	if dataDir == "" {
		dataDir = omapDataDir()
	}
	if dataDir == "" {
		fail("未设置 OMap data 目录，请指定 --data 或设置 QINGHAI_OMAP_DATA / OMAP_DATA_DIR")
	}
	if strings.TrimSpace(*station) == "" || strings.TrimSpace(*segment) == "" {
		fail("point 必须指定 --segment 和 --station")
	}
	if strings.TrimSpace(*brigade) != "韵家口大队" {
		fail("point 当前仅支持韵家口大队")
	}
	category, subfolders, err := parsePointFolder(*folder, *assetType)
	if err != nil {
		fail(err.Error())
	}
	if len(attachments) != 1 {
		fail("每个点必须且只能指定一张图片附件")
	}
	out, err := locator.ExportOmap(locator.OmapPointInput{Station: *station, SegmentID: *segment, Brigade: *brigade, AssetType: category, Subfolders: subfolders, Name: *name, Comment: *comment, Attachments: attachments, OutputDirectory: dataDir, SyncToOmap: true})
	if err != nil {
		fail(err.Error())
	}
	writeResult(out, *jsonOut)
}

func parsePointFolder(folder, assetType string) (string, []string, error) {
	assetType = strings.TrimSpace(assetType)
	folder = strings.TrimSpace(folder)
	if folder == "" {
		if assetType == "" {
			return "", nil, errors.New("必须指定 --folder 或 --asset-type")
		}
		return assetType, nil, nil
	}
	parts := strings.Split(strings.ReplaceAll(folder, `\`, "/"), "/")
	for index, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `<>:"|?*`) || strings.IndexFunc(part, func(r rune) bool { return r < 32 }) >= 0 {
			return "", nil, fmt.Errorf("--folder 必须是大队下的安全相对目录：%q", folder)
		}
		parts[index] = part
	}
	if assetType != "" && assetType != parts[0] {
		return "", nil, fmt.Errorf("--asset-type %q 与 --folder 首级 %q 不一致", assetType, parts[0])
	}
	return parts[0], parts[1:], nil
}

type multiFlag []string

func (m *multiFlag) String() string         { return strings.Join(*m, ",") }
func (m *multiFlag) Set(value string) error { *m = append(*m, value); return nil }

func runScan(args []string, doImport bool) {
	command := "scan"
	description := "扫描档案图片并预览桩号、路段、名称、分类及警告；不写入 OMap。"
	if doImport {
		command = "import"
		description = "只导入新增或变更的档案图片；状态记录在档案根目录。运行前请完全退出 OMap。"
	}
	fs := commandFlags(command, description, "omap-import "+command+" --root 'C:\\Entry\\路产档案' --segment xjk-g6-xiguojing --json")
	root := fs.String("root", "", "路产档案目录")
	segment := fs.String("segment", "", "路段 ID")
	brigade := fs.String("brigade", "韵家口大队", "大队")
	dryRun := fs.Bool("dry-run", false, "只检查，不写入")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	_ = fs.Parse(args)
	if strings.TrimSpace(*root) == "" {
		fail("必须指定 --root")
	}
	items, warnings, err := discover(*root, *segment, *brigade)
	if err != nil {
		fail(err.Error())
	}
	st, _ := loadState(filepath.Join(*root, stateName))
	changed := make([]item, 0, len(items))
	for _, it := range items {
		if st.Sources[it.Path] != it.Fingerprint {
			changed = append(changed, it)
		}
	}
	if !doImport || *dryRun {
		writeResult(map[string]any{"root": *root, "total": len(items), "changed": len(changed), "items": items, "warnings": warnings}, *jsonOut)
		return
	}
	dataDir := omapDataDir()
	if dataDir == "" {
		fail("未设置 OMAP data 目录，请设置 QINGHAI_OMAP_DATA 或 OMAP_DATA_DIR")
	}
	results := make([]any, 0, len(changed))
	failed := 0
	for _, it := range changed {
		attachments := it.Attachments
		if len(attachments) == 0 {
			attachments = []string{it.Path}
		}
		if len(attachments) != 1 {
			failed++
			warnings = append(warnings, fmt.Sprintf("%s: 每个点必须且只能有一张图片", it.Path))
			continue
		}
		out, err := locator.ExportOmap(locator.OmapPointInput{Station: it.Station, SegmentID: it.SegmentID, Brigade: it.Brigade, AssetType: it.AssetType, Name: it.Name, Comment: it.Comment, Attachments: attachments, OutputDirectory: dataDir, SyncToOmap: true})
		if err != nil {
			failed++
			warnings = append(warnings, fmt.Sprintf("%s: %v", it.Path, err))
			continue
		}
		st.Sources[it.Path] = it.Fingerprint
		results = append(results, out)
	}
	st.Omap = inventory(dataDir)
	st.Updated = time.Now().Format(time.RFC3339)
	if err := saveState(filepath.Join(*root, stateName), st); err != nil {
		fail(err.Error())
	}
	writeResult(map[string]any{"root": *root, "scanned": len(items), "imported": len(results), "failed": failed, "results": results, "warnings": warnings}, *jsonOut)
	if failed > 0 {
		os.Exit(1)
	}
}

func runStatus(args []string) {
	fs := commandFlags("status", "比较档案图片与上次导入状态，并列出 OMap data 已变更文件。", "omap-import status --root 'C:\\Entry\\路产档案' --json")
	root := fs.String("root", "", "档案目录")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	_ = fs.Parse(args)
	if *root == "" {
		fail("必须指定 --root")
	}
	st, _ := loadState(filepath.Join(*root, stateName))
	items, warnings, err := discover(*root, "", "韵家口大队")
	if err != nil {
		fail(err.Error())
	}
	changed := 0
	for _, it := range items {
		if st.Sources[it.Path] != it.Fingerprint {
			changed++
		}
	}
	current := inventory(omapDataDir())
	omapChanged := diffInventory(st.Omap, current)
	writeResult(map[string]any{"sourceTotal": len(items), "sourceChanged": changed, "omapChanged": omapChanged, "warnings": warnings}, *jsonOut)
}

func runDelta(args []string) {
	fs := commandFlags("delta", "只复制 OMap data 中新增或变更的文件到目标目录的 data 子目录。", "omap-import delta --destination 'D:\\omap-increment' --data 'C:\\Users\\13421\\Documents\\omap\\data' --json")
	destination := fs.String("destination", "", "增量目录")
	data := fs.String("data", "", "OMap data 目录")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	_ = fs.Parse(args)
	if *destination == "" {
		fail("必须指定 --destination")
	}
	if *data == "" {
		*data = omapDataDir()
	}
	if *data == "" {
		fail("未设置 OMap data 目录")
	}
	statePath := filepath.Join(*destination, stateName)
	st, _ := loadState(statePath)
	current := inventory(*data)
	copied := []string{}
	for rel, meta := range current {
		if old, ok := st.Omap[rel]; ok && old == meta {
			continue
		}
		src := filepath.Join(*data, filepath.FromSlash(rel))
		dst := filepath.Join(*destination, "data", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			fail(err.Error())
		}
		if err := copy(src, dst); err != nil {
			fail(err.Error())
		}
		copied = append(copied, rel)
	}
	st.Omap = current
	st.Updated = time.Now().Format(time.RFC3339)
	if err := saveState(statePath, st); err != nil {
		fail(err.Error())
	}
	sort.Strings(copied)
	writeResult(map[string]any{"copied": copied, "count": len(copied), "source": *data, "destination": *destination}, *jsonOut)
}

func discover(root, forcedSegment, forcedBrigade string) ([]item, []string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	warnings := []string{}
	groups := map[string]*item{}
	manifest := filepath.Join(root, "manifest.json")
	if raw, err := os.ReadFile(manifest); err == nil {
		var listed []item
		if json.Unmarshal(raw, &listed) == nil {
			for _, it := range listed {
				resolveItem(&it, root, forcedSegment, forcedBrigade)
				if !isImagePath(it.Path) {
					warnings = append(warnings, fmt.Sprintf("清单中的非图片文件，跳过：%s", it.Path))
					continue
				}
				if len(it.Attachments) != 1 || !isImagePath(it.Attachments[0]) {
					warnings = append(warnings, fmt.Sprintf("清单必须为每个点指定一张图片，跳过：%s", it.Path))
					continue
				}
				addGroup(groups, it)
			}
			return sortedItems(groups), warnings, nil
		}
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || filepath.Base(path) == stateName || strings.EqualFold(filepath.Base(path), "manifest.json") {
			return nil
		}
		if !isImagePath(path) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		it := item{Path: path}
		if !parseItem(&it, rel, forcedSegment, forcedBrigade) {
			warnings = append(warnings, fmt.Sprintf("未识别桩号，跳过：%s", rel))
			return nil
		}
		it.Fingerprint = fingerprint(path, info)
		addGroup(groups, it)
		return nil
	})
	return sortedItems(groups), warnings, err
}

func parseItem(it *item, rel, forcedSegment, brigade string) bool {
	m := stationRE.FindStringSubmatch(rel)
	if m == nil {
		return false
	}
	code, km, metres := stationParts(m)
	it.SegmentID = forcedSegment
	it.Brigade = brigade
	if it.SegmentID == "" {
		it.SegmentID = segmentFor(code, km*1000+metres, brigade)
	}
	if it.SegmentID == "" {
		return false
	}
	if code == "" {
		for _, c := range locator.CoverageList() {
			if c.SegmentID == it.SegmentID {
				code = c.Code
				break
			}
		}
	}
	it.Station = fmt.Sprintf("%s K%d+%03d", code, km, metres)
	it.AssetType = classify(rel)
	it.Name = deriveName(rel, it.AssetType)
	return true
}

func resolveItem(it *item, root, segment, brigade string) {
	if !filepath.IsAbs(it.Path) {
		it.Path = filepath.Join(root, it.Path)
	}
	if it.SegmentID == "" {
		it.SegmentID = segment
	}
	if it.Brigade == "" {
		it.Brigade = brigade
	}
	if len(it.Attachments) == 0 {
		it.Attachments = []string{it.Path}
	} else {
		for index, attachment := range it.Attachments {
			if !filepath.IsAbs(attachment) {
				it.Attachments[index] = filepath.Join(root, attachment)
			}
		}
	}
	if it.Fingerprint == "" {
		if info, err := os.Stat(it.Path); err == nil {
			it.Fingerprint = fingerprint(it.Path, info)
		}
	}
}
func addGroup(groups map[string]*item, it item) {
	// OMAP native points have one verified attachment slot. Keep each image as
	// its own point so images from different assets at the same station cannot
	// be merged and no ZIP bundle is ever created.
	key := it.Path
	it.Attachments = []string{it.Path}
	it.Fingerprint = aggregateFingerprint(it.Attachments)
	groups[key] = &it
}

func aggregateFingerprint(paths []string) string {
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s\x00%d\x00%d\x00%s\x00", path, info.Size(), info.ModTime().UnixNano(), fingerprint(path, info))
	}
	return hex.EncodeToString(h.Sum(nil))
}
func sortedItems(groups map[string]*item) []item {
	out := make([]item, 0, len(groups))
	for _, it := range groups {
		sort.Strings(it.Attachments)
		out = append(out, *it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
func segmentFor(code string, meter int, brigade string) string {
	for _, c := range locator.CoverageList() {
		if strings.EqualFold(c.Code, code) && (brigade == "" || c.Brigade == brigade) && meter >= parseMeter(c.Start) && meter <= parseMeter(c.End) {
			return c.SegmentID
		}
	}
	return ""
}
func parseMeter(s string) int {
	m := stationRE.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	_, a, b := stationParts(m)
	return a*1000 + b
}

func stationParts(m []string) (string, int, int) {
	if len(m) < 6 {
		return "", 0, 0
	}
	code := strings.ToUpper(m[1])
	kmText, metreText := m[2], m[3]
	if kmText == "" {
		kmText, metreText = m[4], m[5]
	}
	km, _ := atoi(kmText)
	metres, _ := atoi(metreText)
	return code, km, metres
}
func classify(s string) string {
	for _, pair := range []struct{ key, value string }{
		{"行政许可", "行政许可"}, {"建筑控制区内非公路标志牌", "建筑控制区内非公路标志牌"},
		{"公路用地非公路标志牌", "公路用地非公路标志牌"}, {"非公路标志", "公路附属设施标志标牌"},
		{"标志标牌", "公路附属设施标志标牌"}, {"监控", "监控设施"}, {"ETC", "ETC龙门架"},
		{"情报板", "情报板"}, {"高边坡", "高边坡"}, {"安全隐患", "安全隐患"}, {"网格化", "网格化联络表"},
		{"桥", "桥梁"}, {"涵", "涵洞"}, {"隧", "隧道"}, {"收费", "收费站"},
		{"服务区", "服务区、停车区"}, {"停车", "服务区、停车区"}, {"避险", "避险车道"},
		{"劝返", "劝返站点"}, {"跨线桥", "跨线桥"}, {"涉路施工", "涉路施工"},
		{"施工", "涉路施工监管"}, {"通道", "车辆通道"}, {"桩号", "桩号"},
	} {
		if strings.Contains(s, pair.key) {
			return pair.value
		}
	}
	return "公路附属设施标志标牌"
}
func deriveName(rel, typ string) string {
	base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	base = stationRE.ReplaceAllString(base, "")
	base = strings.TrimSpace(strings.Trim(base, "-_+（）() "))
	base = regexp.MustCompile(`^[0-9０-９]+[.、_\-、 ]*`).ReplaceAllString(base, "")
	base = strings.TrimSpace(strings.Trim(base, "-_+（）() "))
	if base == "" {
		parent := filepath.Base(filepath.Dir(rel))
		base = strings.TrimSpace(strings.Trim(stationRE.ReplaceAllString(parent, ""), "-_+（）() "))
	}
	if base == "" {
		base = typ
	}
	return base
}

func isImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".bmp", ".gif", ".tif", ".tiff":
		return true
	default:
		return false
	}
}
func atoi(s string) (int, error) { var n int; _, err := fmt.Sscanf(s, "%d", &n); return n, err }

func loadState(path string) (state, error) {
	st := state{Version: 1, Sources: map[string]string{}, Omap: map[string]fileState{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		return st, err
	}
	if json.Unmarshal(raw, &st) != nil {
		return st, errors.New("状态文件格式无效")
	}
	if st.Sources == nil {
		st.Sources = map[string]string{}
	}
	if st.Omap == nil {
		st.Omap = map[string]fileState{}
	}
	return st, nil
}
func saveState(path string, st state) error {
	raw, _ := json.MarshalIndent(st, "", "  ")
	return os.WriteFile(path, raw, 0o644)
}
func fingerprint(path string, info os.FileInfo) string {
	h := sha256.New()
	f, e := os.Open(path)
	if e == nil {
		_, _ = io.Copy(h, f)
		_ = f.Close()
		return hex.EncodeToString(h.Sum(nil))
	}
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}
func inventory(root string) map[string]fileState {
	out := map[string]fileState{}
	if root == "" {
		return out
	}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(root, path)
		if e == nil {
			out[filepath.ToSlash(rel)] = fileState{info.Size(), info.ModTime().UnixNano()}
		}
		return nil
	})
	return out
}
func diffInventory(a, b map[string]fileState) []string {
	out := []string{}
	for k, v := range b {
		if old, ok := a[k]; !ok || old != v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
func copy(src, dst string) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.Create(dst)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	ce := out.Close()
	if e != nil {
		return e
	}
	return ce
}
func omapDataDir() string {
	for _, k := range []string{"QINGHAI_OMAP_DATA", "OMAP_DATA_DIR"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return filepath.Clean(v)
		}
	}
	return ""
}
func writeResult(v any, jsonOut bool) {
	if jsonOut {
		raw, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(raw))
		return
	}
	raw, _ := json.Marshal(v)
	fmt.Println(string(raw))
}
func fail(msg string) { fmt.Fprintln(os.Stderr, msg); os.Exit(2) }
