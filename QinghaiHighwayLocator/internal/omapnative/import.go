package omapnative

import (
	"crypto/md5"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	databaseFileName = "oobj.odb"
	databasePassword = "ovital318uinkme"
	folderObjectType = 30
	pointObjectType  = 7

	folderNameLengthOffset = 100
	folderNameOffset       = 101
	pointAttachmentOffset  = 40
	pointAttachmentSize    = 48
	pointExtensionOffset   = 56
	pointLatitudeOffset    = 300
	pointLongitudeOffset   = 308
	pointAltitudeOffset    = 316
	pointNameLengthOffset  = 331
	pointNameOffset        = 332
	pointPrefixLength      = 331
	pointSuffixLength      = 6
)

// ImportInput contains the values needed to add one native OMAP point. DataDirectory
// must be the installed OMAP data directory, not an export directory.
type ImportInput struct {
	DataDirectory string
	Brigade       string
	AssetType     string
	Subfolders    []string
	Name          string
	Latitude      float64
	Longitude     float64
	Altitude      *int32
	Comment       string
	Attachments   []string
}

type ImportedAttachment struct {
	SourcePath string `json:"sourcePath"`
	OmapName   string `json:"omapName"`
	ID         uint64 `json:"id"`
}

type ImportResult struct {
	DataFile        string               `json:"dataFile"`
	BackupDirectory string               `json:"backupDirectory"`
	TargetFolder    string               `json:"targetFolder"`
	ObjectID        uint32               `json:"objectId"`
	Attachments     []ImportedAttachment `json:"attachments"`
}

type objectRecord struct {
	userID     int64
	objectID   int64
	parentID   int64
	serverID   int64
	objectType int64
	modifiedAt int64
	childCount int64
	dataHash   int64
	groupHash  int64
	styleHash  int64
	data       []byte
	groupData  []byte
}

type preparedAttachment struct {
	sourcePath string
	name       string
	extension  string
	size       int64
	id         uint64
}

// ErrNativeWriteDisabled is retained for callers compiled against older
// versions. Native writes are enabled after the object layout was matched to
// OMAP-created historical databases.
var ErrNativeWriteDisabled = errors.New("奥维原生 data 写入已暂停")

// Import writes one point directly into the installed OMAP object database.
// OMAP must be fully closed while this operation replaces oobj.odb.
func Import(input ImportInput) (ImportResult, error) {
	return importUnverified(input)
}

// importUnverified contains the transactional native writer used by Import.
func importUnverified(input ImportInput) (ImportResult, error) {
	if err := requireOmapClosed(); err != nil {
		return ImportResult{}, err
	}
	dataDirectory := filepath.Clean(strings.TrimSpace(input.DataDirectory))
	if dataDirectory == "" || dataDirectory == "." {
		return ImportResult{}, errors.New("未找到奥维 data 目录")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ImportResult{}, errors.New("请输入点位名称")
	}
	if len([]byte(name)) > 255 {
		return ImportResult{}, errors.New("点位名称过长，最多 255 字节")
	}
	if len([]byte(input.Comment)) > 32768 {
		return ImportResult{}, errors.New("备注过长，最多 32768 字节")
	}
	if math.IsNaN(input.Latitude) || math.IsNaN(input.Longitude) || input.Latitude < -90 || input.Latitude > 90 || input.Longitude < -180 || input.Longitude > 180 {
		return ImportResult{}, errors.New("经纬度无效")
	}
	category, ok := categoryName(input.AssetType)
	if !ok {
		return ImportResult{}, errors.New("请选择有效的路产类型")
	}
	if err := validateSubfolders(input.Subfolders); err != nil {
		return ImportResult{}, err
	}
	databasePath := filepath.Join(dataDirectory, databaseFileName)
	encrypted, err := os.ReadFile(databasePath)
	if err != nil {
		return ImportResult{}, fmt.Errorf("无法读取奥维对象数据：%w", err)
	}
	plain, err := DecodeDatabase(encrypted, databasePassword)
	if err != nil {
		return ImportResult{}, fmt.Errorf("无法解密奥维对象数据：%w", err)
	}
	if len(plain) < 16 || string(plain[:16]) != "SQLite format 3\x00" {
		return ImportResult{}, errors.New("奥维对象数据格式未识别，未写入")
	}

	attachments, err := prepareAttachments(input.Attachments)
	if err != nil {
		return ImportResult{}, err
	}
	if len(attachments) > 1 {
		return ImportResult{}, errors.New("一个奥维点只能写入一张图片；请为每张图片建立独立点位")
	}

	workDirectory, err := os.MkdirTemp("", "qinghai-omap-import-")
	if err != nil {
		return ImportResult{}, fmt.Errorf("无法创建导入工作区：%w", err)
	}
	defer os.RemoveAll(workDirectory)
	plainPath := filepath.Join(workDirectory, "oobj.sqlite")
	if err := os.WriteFile(plainPath, plain, 0o600); err != nil {
		return ImportResult{}, fmt.Errorf("无法准备奥维对象副本：%w", err)
	}

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(plainPath))
	if err != nil {
		return ImportResult{}, fmt.Errorf("无法打开奥维对象副本：%w", err)
	}
	if err := validateDatabase(db); err != nil {
		db.Close()
		return ImportResult{}, err
	}
	if _, err := db.Exec("PRAGMA journal_mode=DELETE"); err != nil {
		db.Close()
		return ImportResult{}, fmt.Errorf("无法准备奥维对象写入：%w", err)
	}

	now := time.Now().Unix()
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return ImportResult{}, fmt.Errorf("无法建立奥维对象事务：%w", err)
	}
	root, err := findFolder(tx, 0, "收藏夹")
	if err == nil {
		root, err = ensureFolder(tx, root, "西宁高速支队", now)
	}
	brigade := strings.TrimSpace(input.Brigade)
	if brigade == "" {
		brigade = "韵家口大队"
	}
	if err == nil {
		root, err = ensureFolder(tx, root, brigade, now)
	}
	if err == nil {
		root, err = ensureFolder(tx, root, category, now)
	}
	for _, subfolder := range input.Subfolders {
		if err != nil {
			break
		}
		root, err = ensureFolder(tx, root, subfolder, now)
	}
	if err == nil {
		err = repairFolderChain(tx, root, now)
	}
	var point objectRecord
	if err == nil {
		point, err = insertPoint(tx, root, name, input.Latitude, input.Longitude, input.Altitude, input.Comment, attachments, now)
	}
	if err != nil {
		_ = tx.Rollback()
		db.Close()
		return ImportResult{}, fmt.Errorf("无法构建奥维对象：%w", err)
	}
	if err := tx.Commit(); err != nil {
		db.Close()
		return ImportResult{}, fmt.Errorf("无法提交奥维对象：%w", err)
	}
	if err := validateDatabase(db); err != nil {
		db.Close()
		return ImportResult{}, err
	}
	if err := db.Close(); err != nil {
		return ImportResult{}, fmt.Errorf("无法关闭奥维对象副本：%w", err)
	}

	changedPlain, err := os.ReadFile(plainPath)
	if err != nil {
		return ImportResult{}, fmt.Errorf("无法读取写入后的奥维对象副本：%w", err)
	}
	changedEncrypted, err := EncodeDatabase(changedPlain, databasePassword)
	if err != nil {
		return ImportResult{}, fmt.Errorf("无法加密写入后的奥维对象：%w", err)
	}
	if err := verifyEncodedDatabase(changedEncrypted); err != nil {
		return ImportResult{}, err
	}

	backupDirectory, err := backupDatabase(dataDirectory, databasePath)
	if err != nil {
		return ImportResult{}, err
	}
	createdAttachments, err := installAttachments(dataDirectory, attachments)
	if err != nil {
		return ImportResult{}, err
	}
	if err := replaceDatabase(databasePath, changedEncrypted); err != nil {
		removeCreatedFiles(createdAttachments)
		return ImportResult{}, err
	}

	resultAttachments := make([]ImportedAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		resultAttachments = append(resultAttachments, ImportedAttachment{
			SourcePath: attachment.sourcePath,
			OmapName:   attachment.name,
			ID:         attachment.id,
		})
	}
	return ImportResult{
		DataFile:        databasePath,
		BackupDirectory: backupDirectory,
		TargetFolder:    strings.Join(append([]string{"收藏夹", "西宁高速支队", brigade, category}, input.Subfolders...), " > "),
		ObjectID:        uint32(point.objectID),
		Attachments:     resultAttachments,
	}, nil
}

func requireOmapClosed() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	output, err := exec.Command("tasklist", "/FI", "IMAGENAME eq omap.exe", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return fmt.Errorf("无法确认奥维进程状态：%w", err)
	}
	if strings.Contains(strings.ToLower(string(output)), "omap.exe") {
		return errors.New("奥维正在运行，请完全退出奥维后再写入 data")
	}
	return nil
}

func categoryName(assetType string) (string, bool) {
	switch strings.TrimSpace(assetType) {
	case "桥梁", "涵洞", "隧道", "收费站", "服务区、停车区", "避险车道", "劝返站点", "跨线桥", "行政许可", "涉路施工监管", "涉路施工", "车辆通道", "桩号", "监控设施", "ETC龙门架", "情报板", "高边坡", "安全隐患", "网格化联络表", "建筑控制区内非公路标志牌", "公路用地非公路标志牌", "公路附属设施标志标牌":
		return strings.TrimSpace(assetType), true
	case "服务区", "停车区", "停车区、服务区":
		return "服务区、停车区", true
	default:
		return "", false
	}
}

func validateSubfolders(subfolders []string) error {
	for _, name := range subfolders {
		if name == "" || name != strings.TrimSpace(name) || name == "." || name == ".." || len([]byte(name)) > 255 || strings.ContainsAny(name, `<>:"/\|?*`) || strings.IndexFunc(name, func(r rune) bool { return r < 32 }) >= 0 {
			return fmt.Errorf("无效的 OMap 子目录名称：%q", name)
		}
	}
	return nil
}

func validateDatabase(db *sql.DB) error {
	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("无法校验奥维对象数据：%w", err)
	}
	if result != "ok" {
		return fmt.Errorf("奥维对象数据校验失败：%s", result)
	}
	return nil
}

func findFolder(tx *sql.Tx, parentID int64, name string) (objectRecord, error) {
	rows, err := tx.Query(`SELECT userid, objid, parentid, srvid, type, modifytime, allchildcnt,
		md5d, md5g, md5s, data, groupdata FROM object WHERE parentid = ? AND type = ?`, parentID, folderObjectType)
	if err != nil {
		return objectRecord{}, err
	}
	defer rows.Close()
	for rows.Next() {
		record, err := scanObject(rows)
		if err != nil {
			return objectRecord{}, err
		}
		if folderObjectName(record.data) == name {
			return record, nil
		}
	}
	if err := rows.Err(); err != nil {
		return objectRecord{}, err
	}
	return objectRecord{}, sql.ErrNoRows
}

func ensureFolder(tx *sql.Tx, parent objectRecord, name string, modifiedAt int64) (objectRecord, error) {
	folder, err := findFolder(tx, parent.objectID, name)
	if err == nil {
		return folder, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return objectRecord{}, err
	}
	template, err := findFolderTemplate(tx, name)
	if err != nil {
		return objectRecord{}, err
	}
	objectID, err := newObjectID(tx)
	if err != nil {
		return objectRecord{}, err
	}
	data, err := makeFolderData(template.data, name, modifiedAt)
	if err != nil {
		return objectRecord{}, err
	}
	folder = objectRecord{
		userID:     parent.userID,
		objectID:   objectID,
		parentID:   parent.objectID,
		serverID:   newServerID(),
		objectType: folderObjectType,
		modifiedAt: modifiedAt,
		childCount: 0,
		dataHash:   objectHash(data),
		groupHash:  objectHash(nil),
		// A newly-created local folder has no recursive state hash. The
		// template's md5s belongs to its existing children and must not be
		// copied into the new object.
		styleHash: 0,
		data:      data,
		groupData: nil,
	}
	if err := insertObject(tx, folder); err != nil {
		return objectRecord{}, err
	}
	if err := addChild(tx, parent, folder.objectID, modifiedAt); err != nil {
		return objectRecord{}, err
	}
	return folder, nil
}

// repairFolderChain normalizes the fields that OMAP derives from a folder's
// actual child index. It also repairs directories created by earlier builds
// before this invariant was implemented; no child objects are changed.
func repairFolderChain(tx *sql.Tx, leaf objectRecord, modifiedAt int64) error {
	current := leaf
	for {
		groupData := current.groupData
		if len(groupData)%4 != 0 {
			return errors.New("奥维目录子项索引格式异常")
		}
		data := append([]byte(nil), current.data...)
		if len(data) < 20 {
			return errors.New("奥维目录数据格式异常")
		}
		binary.LittleEndian.PutUint32(data[16:20], uint32(len(groupData)/4))
		groupHash := objectHash(groupData)
		if _, err := tx.Exec(`UPDATE object SET data = ?, md5d = ?, md5g = ?, modifytime = ?
			WHERE userid = ? AND objid = ?`, data, objectHash(data), groupHash, modifiedAt, current.userID, current.objectID); err != nil {
			return err
		}
		if current.parentID == 0 {
			return nil
		}
		parent, err := findObject(tx, current.parentID)
		if err != nil {
			return err
		}
		current = parent
	}
}

func findFolderTemplate(tx *sql.Tx, name string) (objectRecord, error) {
	rows, err := tx.Query(`SELECT userid, objid, parentid, srvid, type, modifytime, allchildcnt,
		md5d, md5g, md5s, data, groupdata FROM object WHERE type = ? ORDER BY objid`, folderObjectType)
	if err != nil {
		return objectRecord{}, err
	}
	defer rows.Close()
	var fallback objectRecord
	for rows.Next() {
		record, err := scanObject(rows)
		if err != nil {
			return objectRecord{}, err
		}
		folderName := folderObjectName(record.data)
		if fallback.objectID == 0 && folderName != "收藏夹" && folderName != "西宁高速支队" {
			fallback = record
		}
		if folderName == name {
			return record, nil
		}
	}
	if err := rows.Err(); err != nil {
		return objectRecord{}, err
	}
	if fallback.objectID == 0 {
		return objectRecord{}, errors.New("奥维目录模板不存在")
	}
	return fallback, nil
}

func insertPoint(tx *sql.Tx, parent objectRecord, name string, latitude, longitude float64, altitude *int32, comment string, attachments []preparedAttachment, modifiedAt int64) (objectRecord, error) {
	template, err := findPointTemplate(tx)
	if err != nil {
		return objectRecord{}, err
	}
	objectID, err := newObjectID(tx)
	if err != nil {
		return objectRecord{}, err
	}
	data, err := makePointData(template.data, name, latitude, longitude, altitude, comment, attachments, modifiedAt)
	if err != nil {
		return objectRecord{}, err
	}
	point := objectRecord{
		userID:     parent.userID,
		objectID:   objectID,
		parentID:   parent.objectID,
		serverID:   newServerID(),
		objectType: pointObjectType,
		modifiedAt: modifiedAt,
		childCount: 0,
		dataHash:   objectHash(data),
		groupHash:  0,
		styleHash:  0,
		data:       data,
		groupData:  nil,
	}
	if err := insertObject(tx, point); err != nil {
		return objectRecord{}, err
	}
	if err := addChild(tx, parent, point.objectID, modifiedAt); err != nil {
		return objectRecord{}, err
	}
	return point, nil
}

func findPointTemplate(tx *sql.Tx) (objectRecord, error) {
	row := tx.QueryRow(`SELECT userid, objid, parentid, srvid, type, modifytime, allchildcnt,
		md5d, md5g, md5s, data, groupdata FROM object WHERE type = ? ORDER BY objid LIMIT 1`, pointObjectType)
	return scanObject(row)
}

func addChild(tx *sql.Tx, parent objectRecord, childID, modifiedAt int64) error {
	groupData, err := appendChildID(parent.groupData, childID)
	if err != nil {
		return err
	}
	data := append([]byte(nil), parent.data...)
	if len(data) >= 20 {
		binary.LittleEndian.PutUint32(data[16:20], uint32(len(groupData)/4))
	}
	if _, err := tx.Exec(`UPDATE object SET data = ?, md5d = ?, groupdata = ?, md5g = ?, allchildcnt = allchildcnt + 1, modifytime = ?
		WHERE userid = ? AND objid = ?`, data, objectHash(data), groupData, objectHash(groupData), modifiedAt, parent.userID, parent.objectID); err != nil {
		return err
	}
	ancestorID := parent.parentID
	for ancestorID != 0 {
		ancestor, err := findObject(tx, ancestorID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE object SET allchildcnt = allchildcnt + 1, modifytime = ? WHERE userid = ? AND objid = ?`,
			modifiedAt, ancestor.userID, ancestor.objectID); err != nil {
			return err
		}
		ancestorID = ancestor.parentID
	}
	return nil
}

func findObject(tx *sql.Tx, objectID int64) (objectRecord, error) {
	row := tx.QueryRow(`SELECT userid, objid, parentid, srvid, type, modifytime, allchildcnt,
		md5d, md5g, md5s, data, groupdata FROM object WHERE objid = ? LIMIT 1`, objectID)
	return scanObject(row)
}

type objectScanner interface {
	Scan(dest ...any) error
}

func scanObject(scanner objectScanner) (objectRecord, error) {
	var record objectRecord
	var dataHash, groupHash, styleHash sql.NullInt64
	if err := scanner.Scan(&record.userID, &record.objectID, &record.parentID, &record.serverID, &record.objectType,
		&record.modifiedAt, &record.childCount, &dataHash, &groupHash, &styleHash, &record.data, &record.groupData); err != nil {
		return objectRecord{}, err
	}
	record.dataHash = dataHash.Int64
	record.groupHash = groupHash.Int64
	record.styleHash = styleHash.Int64
	return record, nil
}

func insertObject(tx *sql.Tx, record objectRecord) error {
	_, err := tx.Exec(`INSERT INTO object (userid, objid, parentid, srvid, type, modifytime, allchildcnt, md5d, md5g, md5s, data, groupdata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.userID, record.objectID, record.parentID, record.serverID,
		record.objectType, record.modifiedAt, record.childCount, record.dataHash, record.groupHash, record.styleHash, record.data, record.groupData)
	return err
}

func folderObjectName(data []byte) string {
	if len(data) <= folderNameLengthOffset {
		return ""
	}
	length := int(data[folderNameLengthOffset])
	if folderNameOffset+length > len(data) {
		return ""
	}
	return string(data[folderNameOffset : folderNameOffset+length])
}

func makeFolderData(template []byte, name string, modifiedAt int64) ([]byte, error) {
	if len(template) <= folderNameLengthOffset {
		return nil, errors.New("奥维目录模板格式不完整")
	}
	nameBytes := []byte(name)
	if len(nameBytes) > 255 {
		return nil, errors.New("目录名称过长")
	}
	data := append([]byte(nil), template[:folderNameLengthOffset]...)
	if len(data) >= 20 {
		binary.LittleEndian.PutUint32(data[16:20], 0)
	}
	putFolderTime(data, modifiedAt)
	data = append(data, byte(len(nameBytes)))
	data = append(data, nameBytes...)
	data = append(data, 0)
	return data, nil
}

func makePointData(template []byte, name string, latitude, longitude float64, altitude *int32, comment string, attachments []preparedAttachment, modifiedAt int64) ([]byte, error) {
	if len(template) < pointNameOffset {
		return nil, errors.New("奥维点模板格式不完整")
	}
	nameBytes := []byte(name)
	if len(nameBytes) > 255 {
		return nil, errors.New("点位名称过长")
	}
	if len(attachments) > 1 {
		return nil, errors.New("原生点位多附件编码尚未验证")
	}
	data := append([]byte(nil), template[:pointPrefixLength]...)
	putObjectTime(data, modifiedAt)
	// OMAP's native point editor uses this fixed metadata tuple for an object
	// that can be moved/edited. Older databases may provide a locked template;
	// normalize it for every point emitted by this writer.
	if len(data) >= 276 {
		data[272], data[273], data[274], data[275] = 4, 0, 1, 0
	}
	for offset := pointAttachmentOffset; offset < pointExtensionOffset+16; offset++ {
		data[offset] = 0
	}
	if len(attachments) > 0 {
		first := attachments[0]
		binary.LittleEndian.PutUint64(data[pointAttachmentOffset:pointAttachmentOffset+8], first.id)
		if first.size > math.MaxUint32 {
			return nil, fmt.Errorf("附件超过奥维单文件 4 GB 限制：%s", filepath.Base(first.sourcePath))
		}
		binary.LittleEndian.PutUint32(data[pointAttachmentSize:pointAttachmentSize+4], uint32(first.size))
		if len(first.extension) > 15 {
			return nil, fmt.Errorf("附件扩展名过长：%s", first.extension)
		}
		copy(data[pointExtensionOffset:pointExtensionOffset+16], first.extension)
	}
	binary.LittleEndian.PutUint64(data[pointLatitudeOffset:pointLatitudeOffset+8], math.Float64bits(latitude))
	binary.LittleEndian.PutUint64(data[pointLongitudeOffset:pointLongitudeOffset+8], math.Float64bits(longitude))
	if altitude != nil {
		binary.LittleEndian.PutUint32(data[pointAltitudeOffset:pointAltitudeOffset+4], uint32(*altitude))
	}
	data = append(data, byte(len(nameBytes)))
	data = append(data, nameBytes...)
	data = append(data, make([]byte, pointSuffixLength)...)
	if comment = strings.TrimSpace(comment); comment != "" {
		data = append(data, []byte(comment)...)
		data = append(data, 0)
	}
	return data, nil
}

func putObjectTime(data []byte, modifiedAt int64) {
	if len(data) >= 8 {
		binary.LittleEndian.PutUint32(data[4:8], uint32(modifiedAt))
	}
}

func putFolderTime(data []byte, modifiedAt int64) {
	if len(data) >= 72 {
		binary.LittleEndian.PutUint32(data[64:68], uint32(modifiedAt))
	}
}

func appendChildID(groupData []byte, childID int64) ([]byte, error) {
	if childID <= 0 || childID > math.MaxUint32 {
		return nil, errors.New("奥维对象 ID 超出本地目录范围")
	}
	if len(groupData)%4 != 0 {
		return nil, errors.New("奥维目录子项索引格式异常")
	}
	for offset := 0; offset < len(groupData); offset += 4 {
		if uint32(childID) == binary.LittleEndian.Uint32(groupData[offset:offset+4]) {
			return nil, errors.New("奥维目录已有相同对象 ID")
		}
	}
	// OMAP places the newest child first in the folder index. Keeping this
	// order matters because the index is part of the folder integrity state.
	updated := make([]byte, 0, len(groupData)+4)
	var idBytes [4]byte
	binary.LittleEndian.PutUint32(idBytes[:], uint32(childID))
	updated = append(updated, idBytes[:]...)
	return append(updated, groupData...), nil
}

func objectHash(data []byte) int64 {
	sum := md5.Sum(data)
	return int64(binary.LittleEndian.Uint32(sum[:4]) & math.MaxInt32)
}

func newObjectID(tx *sql.Tx) (int64, error) {
	for attempts := 0; attempts < 64; attempts++ {
		var bytes [4]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			return 0, err
		}
		candidate := int64(binary.LittleEndian.Uint32(bytes[:]) & math.MaxInt32)
		if candidate < 1000 {
			continue
		}
		var found int
		if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM object WHERE objid = ?)", candidate).Scan(&found); err != nil {
			return 0, err
		}
		if found == 0 {
			return candidate, nil
		}
	}
	return 0, errors.New("无法生成未占用的奥维对象 ID")
}

func newServerID() int64 {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return int64(binary.LittleEndian.Uint64(bytes[:]) & math.MaxInt64)
	}
	return time.Now().UnixNano() & math.MaxInt64
}

func prepareAttachments(paths []string) ([]preparedAttachment, error) {
	attachments := make([]preparedAttachment, 0, len(paths))
	seen := make(map[string]struct{})
	for _, raw := range paths {
		path := filepath.Clean(strings.TrimSpace(raw))
		if path == "" || path == "." {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("附件不可读取：%s", filepath.Base(path))
		}
		key := strings.ToLower(path)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		id, err := newAttachmentID()
		if err != nil {
			return nil, err
		}
		extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(info.Name())), ".")
		if extension == "" {
			return nil, fmt.Errorf("图片没有扩展名：%s", info.Name())
		}
		if !isImageExtension(extension) {
			return nil, fmt.Errorf("只允许图片附件，已拒绝：%s", info.Name())
		}
		attachments = append(attachments, preparedAttachment{sourcePath: path, extension: extension, size: info.Size(), id: id})
	}
	return attachments, nil
}

func isImageExtension(extension string) bool {
	switch strings.ToLower(strings.TrimPrefix(extension, ".")) {
	case "jpg", "jpeg", "png", "webp", "bmp", "gif", "tif", "tiff":
		return true
	default:
		return false
	}
}

func newAttachmentID() (uint64, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(bytes[:]) & math.MaxInt64, nil
}

func backupDatabase(dataDirectory, databasePath string) (string, error) {
	omapDirectory := filepath.Dir(dataDirectory)
	stamp := time.Now().Format("20060102-150405.000")
	backupDirectory := filepath.Join(omapDirectory, "station_locator_backup", stamp+"-before-native-import")
	if err := os.MkdirAll(backupDirectory, 0o755); err != nil {
		return "", fmt.Errorf("无法创建奥维数据备份目录：%w", err)
	}
	if err := copyFile(databasePath, filepath.Join(backupDirectory, databaseFileName), false); err != nil {
		return "", fmt.Errorf("无法备份奥维对象数据：%w", err)
	}
	configPath := filepath.Join(dataDirectory, "ocfg.odb")
	if _, err := os.Stat(configPath); err == nil {
		if err := copyFile(configPath, filepath.Join(backupDirectory, "ocfg.odb"), false); err != nil {
			return "", fmt.Errorf("无法备份奥维配置数据：%w", err)
		}
	}
	return backupDirectory, nil
}

func installAttachments(dataDirectory string, attachments []preparedAttachment) ([]string, error) {
	if len(attachments) == 0 {
		return nil, nil
	}
	directory := filepath.Join(dataDirectory, "attachment")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("无法创建奥维附件目录：%w", err)
	}
	created := make([]string, 0, len(attachments))
	for index := range attachments {
		attachment := &attachments[index]
		stem := strings.TrimSuffix(filepath.Base(attachment.sourcePath), filepath.Ext(attachment.sourcePath))
		stem = sanitizeAttachmentStem(stem)
		attachment.name = fmt.Sprintf("%s(%d).%s", stem, attachment.id, attachment.extension)
		destination := filepath.Join(directory, attachment.name)
		if err := copyFile(attachment.sourcePath, destination, true); err != nil {
			removeCreatedFiles(created)
			return nil, fmt.Errorf("无法写入奥维附件 %s：%w", filepath.Base(attachment.sourcePath), err)
		}
		created = append(created, destination)
	}
	return created, nil
}

func sanitizeAttachmentStem(stem string) string {
	stem = strings.TrimSpace(stem)
	if stem == "" {
		return "附件"
	}
	var builder strings.Builder
	for _, value := range stem {
		if strings.ContainsRune(`<>:"/\\|?*`, value) || value < 32 {
			builder.WriteRune('_')
			continue
		}
		builder.WriteRune(value)
	}
	return strings.TrimRight(strings.TrimSpace(builder.String()), ".")
}

func replaceDatabase(databasePath string, encrypted []byte) error {
	directory := filepath.Dir(databasePath)
	stamp := time.Now().Format("20060102-150405.000000000")
	candidatePath := filepath.Join(directory, ".oobj.station-locator-"+stamp+".new")
	previousPath := filepath.Join(directory, ".oobj.station-locator-"+stamp+".previous")
	if err := writeFileSync(candidatePath, encrypted, 0o600); err != nil {
		return fmt.Errorf("无法写入奥维对象候选数据：%w", err)
	}
	if err := os.Rename(databasePath, previousPath); err != nil {
		_ = os.Remove(candidatePath)
		return fmt.Errorf("无法替换奥维对象数据，请先完全退出奥维：%w", err)
	}
	if err := os.Rename(candidatePath, databasePath); err != nil {
		_ = os.Rename(previousPath, databasePath)
		_ = os.Remove(candidatePath)
		return fmt.Errorf("无法安装新的奥维对象数据，原文件已恢复：%w", err)
	}
	if err := os.Remove(previousPath); err != nil {
		return fmt.Errorf("奥维对象已写入，但无法清理替换临时文件：%w", err)
	}
	return nil
}

func verifyEncodedDatabase(encrypted []byte) error {
	plain, err := DecodeDatabase(encrypted, databasePassword)
	if err != nil {
		return err
	}
	if len(plain) < 16 || string(plain[:16]) != "SQLite format 3\x00" {
		return errors.New("写入后的奥维数据未能通过加密回读校验")
	}
	workDirectory, err := os.MkdirTemp("", "qinghai-omap-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDirectory)
	path := filepath.Join(workDirectory, "verify.sqlite")
	if err := os.WriteFile(path, plain, 0o600); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	return validateDatabase(db)
}

func copyFile(source, destination string, exclusive bool) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if exclusive {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	out, err := os.OpenFile(destination, flags, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(destination)
		return copyErr
	}
	return closeErr
}

func writeFileSync(path string, contents []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		_ = os.Remove(path)
		return err
	}
	return file.Close()
}

func removeCreatedFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
