package locator

import (
	"archive/zip"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"stationnum2omap/internal/omapnative"
	"strconv"
	"strings"
	"time"
)

// OmapPointInput describes one point to exchange with OMAP. Attachment paths
// are local files selected by the native Windows file dialog.
type OmapPointInput struct {
	Station         string   `json:"station"`
	Brigade         string   `json:"brigade"`
	SegmentID       string   `json:"segmentId"`
	AssetType       string   `json:"assetType"`
	Name            string   `json:"name"`
	Attachments     []string `json:"attachments"`
	OutputDirectory string   `json:"outputDirectory"`
	SyncToOmap      bool     `json:"syncToOmap"`
}

// OmapRangeInput describes an attachment-free whole-kilometre import.
// StartStation and EndStation accept values such as "1766" or "K1766".
type OmapRangeInput struct {
	Route           string `json:"route"`
	Brigade         string `json:"brigade"`
	SegmentID       string `json:"segmentId"`
	StartStation    string `json:"startStation"`
	EndStation      string `json:"endStation"`
	Name            string `json:"name"`
	OutputDirectory string `json:"outputDirectory"`
}

type OmapRangeResult struct {
	TargetFolder string   `json:"targetFolder"`
	ObjectIDs    []uint32 `json:"objectIds"`
	FirstStation string   `json:"firstStation"`
	LastStation  string   `json:"lastStation"`
	Written      int      `json:"written"`
	Skipped      int      `json:"skipped"`
}

type OmapAttachment struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	OmapName string `json:"omapName,omitempty"`
	Size     int64  `json:"size"`
}

type OmapExportResult struct {
	PackagePath             string           `json:"packagePath"`
	OVJSNPath               string           `json:"ovjsnPath"`
	OVKMLPath               string           `json:"ovkmlPath"`
	ManifestPath            string           `json:"manifestPath"`
	TargetFolder            string           `json:"targetFolder"`
	ObjectID                uint32           `json:"objectId"`
	AttachmentQty           int              `json:"attachmentQty"`
	OmapAttachmentQty       int              `json:"omapAttachmentQty"`
	OmapAttachmentDirectory string           `json:"omapAttachmentDirectory,omitempty"`
	Attachments             []OmapAttachment `json:"attachments"`
	DataFile                string           `json:"dataFile,omitempty"`
	BackupDirectory         string           `json:"backupDirectory,omitempty"`
	NativeImported          bool             `json:"nativeImported"`
	Result                  Result           `json:"result"`
}

var assetTypes = map[string]string{
	"桥梁":   "桥梁",
	"隧道":   "隧道",
	"服务区":  "服务区",
	"桩号":   "桩号",
	"行政许可": "行政许可",
	"收费站":  "收费站",
}

// DefaultOmapExportDirectory is deliberately outside OMAP's encrypted data
// files. OMAP can import the generated OVJSN from this directory safely.
func DefaultOmapExportDirectory() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Documents", "omap", "exports", "西宁高速支队")
	}
	return filepath.Join("exports", "西宁高速支队")
}

// DefaultOmapDataDirectory is the installed OMAP data location. Native point
// writes target this directory while OMAP is closed.
func DefaultOmapDataDirectory() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Documents", "omap", "data")
	}
	return filepath.Join("omap", "data")
}

// DefaultOmapAttachmentDirectory is the local attachment library used by OMAP.
// Its files are named by OMAP as "source name(object id).extension".
func DefaultOmapAttachmentDirectory() string {
	return OmapAttachmentDirectory(DefaultOmapDataDirectory())
}

func OmapAttachmentDirectory(dataDirectory string) string {
	return filepath.Join(filepath.Clean(strings.TrimSpace(dataDirectory)), "attachment")
}

// ExportOmap writes one native point into the installed OMAP object database.
// The historical method name is preserved for the Wails bridge. OMAP must be
// closed so its object file can be replaced safely.
func ExportOmap(input OmapPointInput) (OmapExportResult, error) {
	if _, ok := assetTypes[strings.TrimSpace(input.AssetType)]; !ok {
		return OmapExportResult{}, fmt.Errorf("请选择路产类型")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" && strings.TrimSpace(input.AssetType) != "桩号" {
		return OmapExportResult{}, fmt.Errorf("请输入路产或 Ping 点名称")
	}
	result, err := LocateForSegment(input.SegmentID, input.Station)
	if err != nil {
		return OmapExportResult{}, err
	}

	attachments := make([]OmapAttachment, 0, len(input.Attachments))
	seen := make(map[string]struct{})
	for _, raw := range input.Attachments {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		info, statErr := os.Stat(path)
		if statErr != nil || info.IsDir() {
			return OmapExportResult{}, fmt.Errorf("附件不可读取：%s", filepath.Base(path))
		}
		base := filepath.Base(path)
		if _, exists := seen[strings.ToLower(base)]; exists {
			continue
		}
		seen[strings.ToLower(base)] = struct{}{}
		attachments = append(attachments, OmapAttachment{Name: base, Path: path, Size: info.Size()})
	}

	dataDirectory := strings.TrimSpace(input.OutputDirectory)
	if dataDirectory == "" {
		dataDirectory = DefaultOmapDataDirectory()
	}
	nativeResult, err := omapnative.Import(omapnative.ImportInput{
		DataDirectory: dataDirectory,
		Brigade:       input.Brigade,
		AssetType:     input.AssetType,
		Name:          objectName(name, result, input.AssetType),
		Latitude:      result.Latitude,
		Longitude:     result.Longitude,
		Attachments:   input.Attachments,
	})
	if err != nil {
		return OmapExportResult{}, err
	}
	byPath := make(map[string]string, len(nativeResult.Attachments))
	for _, attachment := range nativeResult.Attachments {
		byPath[attachment.SourcePath] = attachment.OmapName
	}
	for index := range attachments {
		attachments[index].OmapName = byPath[attachments[index].Path]
	}
	return OmapExportResult{
		TargetFolder:            nativeResult.TargetFolder,
		ObjectID:                nativeResult.ObjectID,
		AttachmentQty:           len(attachments),
		OmapAttachmentQty:       len(nativeResult.Attachments),
		OmapAttachmentDirectory: OmapAttachmentDirectory(dataDirectory),
		Attachments:             attachments,
		DataFile:                nativeResult.DataFile,
		BackupDirectory:         nativeResult.BackupDirectory,
		NativeImported:          true,
		Result:                  result,
	}, nil
}

// ExportOmapRange writes one attachment-free object for each valid whole
// kilometre in the requested inclusive range. Each point is validated by the
// same calibrated locator used by the map before the first write starts.
func ExportOmapRange(input OmapRangeInput) (OmapRangeResult, error) {
	route := strings.ToUpper(strings.TrimSpace(input.Route))
	if route == "" {
		return OmapRangeResult{}, fmt.Errorf("请选择线路")
	}
	start, err := parseRangeKilometre(input.StartStation)
	if err != nil {
		return OmapRangeResult{}, fmt.Errorf("起始桩号无效：%w", err)
	}
	end, err := parseRangeKilometre(input.EndStation)
	if err != nil {
		return OmapRangeResult{}, fmt.Errorf("结束桩号无效：%w", err)
	}
	if end < start {
		return OmapRangeResult{}, fmt.Errorf("结束桩号必须不小于起始桩号")
	}
	if end-start > 1000 {
		return OmapRangeResult{}, fmt.Errorf("一次最多生成 1001 个整公里点")
	}
	type candidate struct {
		station string
		result  Result
	}
	candidates := make([]candidate, 0, end-start+1)
	skipped := 0
	for kilometre := start; kilometre <= end; kilometre++ {
		station := fmt.Sprintf("%s K%d+000", route, kilometre)
		result, locateErr := LocateForSegment(input.SegmentID, station)
		if locateErr != nil {
			skipped++
			continue
		}
		candidates = append(candidates, candidate{station: station, result: result})
	}
	if len(candidates) == 0 {
		return OmapRangeResult{}, fmt.Errorf("所选范围没有落在 %s 的已收录覆盖区间内", route)
	}
	ids := make([]uint32, 0, len(candidates))
	var targetFolder string
	for _, item := range candidates {
		result, exportErr := ExportOmap(OmapPointInput{
			Station:         item.station,
			Brigade:         input.Brigade,
			SegmentID:       input.SegmentID,
			AssetType:       "桩号",
			OutputDirectory: input.OutputDirectory,
			SyncToOmap:      true,
		})
		if exportErr != nil {
			return OmapRangeResult{}, fmt.Errorf("写入 %s 失败（已写入 %d 个）：%w", item.station, len(ids), exportErr)
		}
		ids = append(ids, result.ObjectID)
		targetFolder = result.TargetFolder
	}
	return OmapRangeResult{
		TargetFolder: targetFolder,
		ObjectIDs:    ids,
		FirstStation: candidates[0].result.Route + " " + candidates[0].result.Station,
		LastStation:  candidates[len(candidates)-1].result.Route + " " + candidates[len(candidates)-1].result.Station,
		Written:      len(ids),
		Skipped:      skipped,
	}, nil
}

func parseRangeKilometre(value string) (int, error) {
	trimmed := strings.TrimSpace(strings.ToUpper(value))
	trimmed = strings.TrimPrefix(trimmed, "K")
	if trimmed == "" {
		return 0, fmt.Errorf("不能为空")
	}
	for _, r := range trimmed {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("请输入整数公里，例如 1766")
		}
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("请输入整数公里，例如 1766")
	}
	return parsed, nil
}

func objectName(name string, result Result, assetType string) string {
	station := result.Route + " " + result.Station
	if strings.TrimSpace(assetType) == "桩号" {
		return station
	}
	if strings.HasSuffix(name, station) {
		return name
	}
	return strings.TrimSpace(name) + " " + station
}

func newObjectID() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err == nil {
		id := binary.LittleEndian.Uint32(b[:]) & 0x7fffffff
		if id > 1000 {
			return id
		}
	}
	return uint32(time.Now().UnixNano() & 0x7fffffff)
}

func sanitizeFileName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if strings.ContainsRune(`<>:"/\\|?*`, r) || r < 32 {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func syncOmapAttachments(attachments []OmapAttachment, objectID uint32, directory string) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("无法创建奥维附件目录：%w", err)
	}
	created := make([]string, 0, len(attachments))
	for index := range attachments {
		attachment := &attachments[index]
		attachment.OmapName = omapAttachmentName(attachment.Name, objectID)
		destination := filepath.Join(directory, attachment.OmapName)
		if _, err := os.Lstat(destination); err == nil {
			removeFiles(created)
			return fmt.Errorf("奥维附件已存在：%s", attachment.OmapName)
		} else if !errors.Is(err, os.ErrNotExist) {
			removeFiles(created)
			return fmt.Errorf("无法检查奥维附件目录：%w", err)
		}
		if err := copyFileExclusive(attachment.Path, destination); err != nil {
			removeFiles(created)
			return fmt.Errorf("同步奥维附件 %s 失败：%w", attachment.Name, err)
		}
		created = append(created, destination)
	}
	return nil
}

func removeFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}

func omapAttachmentName(name string, objectID uint32) string {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	return fmt.Sprintf("%s(%d)%s", stem, objectID, extension)
}

func copyFileExclusive(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	return closeErr
}

type ovjsnDocument struct {
	Version  string        `json:"Version"`
	Type     int           `json:"Type"`
	ObjItems []ovjsnObject `json:"ObjItems"`
}
type ovjsnObject struct {
	Type     int        `json:"Type"`
	ObjID    uint32     `json:"ObjID"`
	ParentID int        `json:"ParentID"`
	TMModify string     `json:"tmModify"`
	Object   ovjsnPoint `json:"Object"`
}
type ovjsnPoint struct {
	Name         string      `json:"Name"`
	Type         int         `json:"Type"`
	Comment      string      `json:"Comment"`
	ObjectDetail ovjsnDetail `json:"ObjectDetail"`
}
type ovjsnDetail struct {
	Lat       float64      `json:"Lat"`
	Lng       float64      `json:"Lng"`
	Gcj02     int          `json:"Gcj02"`
	Altitude  float64      `json:"Altitude"`
	EditMode  int          `json:"EditMode"`
	TxtType   int          `json:"TxtType"`
	ShowLevel int          `json:"ShowLevel"`
	Time      string       `json:"Time"`
	SignPic   ovjsnSignPic `json:"SignPic"`
}
type ovjsnSignPic struct {
	SignPic int `json:"SignPic"`
}

func marshalOVJSN(id uint32, name, comment string, result Result) ([]byte, error) {
	now := time.Now().Format("2006/01/02 15:04:05")
	doc := ovjsnDocument{Version: "V10.6.2", Type: 1, ObjItems: []ovjsnObject{{Type: 7, ObjID: id, ParentID: 1, TMModify: now, Object: ovjsnPoint{Name: name, Type: 7, Comment: comment, ObjectDetail: ovjsnDetail{Lat: result.Latitude, Lng: result.Longitude, Gcj02: 0, Altitude: 0, EditMode: 0, TxtType: 1, ShowLevel: 1, Time: now, SignPic: ovjsnSignPic{SignPic: 4}}}}}}
	return json.MarshalIndent(doc, "", "  ")
}

type kmlDocument struct {
	XMLName xml.Name `xml:"kml"`
	XMLNS   string   `xml:"xmlns,attr"`
	Doc     kmlDoc   `xml:"Document"`
}
type kmlDoc struct {
	Name      string       `xml:"name"`
	Placemark kmlPlacemark `xml:"Placemark"`
}
type kmlPlacemark struct {
	Name        string   `xml:"name"`
	Description string   `xml:"description,omitempty"`
	CoordType   string   `xml:"OvCoordType"`
	Point       kmlPoint `xml:"Point"`
}
type kmlPoint struct {
	Coordinates string `xml:"coordinates"`
}

func marshalOVKML(name, comment string, result Result) ([]byte, error) {
	doc := kmlDocument{XMLNS: "http://www.opengis.net/kml/2.2", Doc: kmlDoc{Name: name, Placemark: kmlPlacemark{Name: name, Description: comment, CoordType: "CGCS2000", Point: kmlPoint{Coordinates: fmt.Sprintf("%.7f,%.7f,0", result.Longitude, result.Latitude)}}}}
	header := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	body, err := xml.MarshalIndent(doc, "", "  ")
	return append(header, body...), err
}

func buildComment(result Result, assetType, name string, attachments []OmapAttachment) string {
	var b strings.Builder
	fmt.Fprintf(&b, "路产类型：%s\n名称：%s\n桩号：%s %s\n纬度：%.7f\n经度：%.7f\n坐标系：WGS-84 / CGCS2000\n", assetType, name, result.Route, result.Station, result.Latitude, result.Longitude)
	if len(attachments) == 0 {
		b.WriteString("附件：无")
		return b.String()
	}
	b.WriteString("附件：\n")
	for _, item := range attachments {
		fmt.Fprintf(&b, "- %s (%d bytes)\n", item.Name, item.Size)
	}
	return strings.TrimSpace(b.String())
}

func zipDirectory(source, destination string) error {
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()
	archive := zip.NewWriter(out)
	defer archive.Close()
	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
