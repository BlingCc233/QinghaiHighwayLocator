# 给路产档案 AI agent 的执行提示词

你负责把指定大队的路产档案整理并导入 OMap。最终结果必须满足：每张图片对应一个可拖动的 OMap 点，点位落在正确桩号换算出的经纬度，点名包含路产名称和完整桩号，目录归属正确，备注写入点自身的备注字段，重复执行只处理新增或变更图片。

## 环境和工具

1. 确认 OMap 已完全退出。写入期间不能运行 `omap.exe`。
2. 在当前电脑设置环境变量，值是 OMap 的 `data` 目录，不是 `omap.exe` 所在目录：

```powershell
$env:QINGHAI_OMAP_DATA = 'C:\Users\13421\Documents\omap\data'
```

也支持变量名 `OMAP_DATA_DIR`。CLI 位于项目根目录，可用 `go run ./cmd/omap-import ...`；有编译产物时使用 `omap-import.exe`。

单个点也可以直接导入：

```powershell
go run ./cmd/omap-import point --segment xjk-g6-xiguojing --station 'G6 K1807+228' --folder '行政许可/跨越公路' --name '跨越公路架设电缆' --comment '许可编号：青交许字〔2026〕1号；现场备注：右幅' --attachment '.\57+060.jpg' --data 'C:\Users\13421\Documents\omap\data' --json
```

随时运行 `go run ./cmd/omap-import -h` 或 `go run ./cmd/omap-import help point` 查看完整命令帮助；`point -h`、`import -h` 等也适用。`--data` 优先于环境变量。`--folder` 是 `收藏夹 > 西宁高速支队 > 韵家口大队` 下的相对路径，首级必须是路产类型；例如 `行政许可/跨越公路`。若无子目录，也可用 `--asset-type 桥梁`。不要传入磁盘绝对路径或其它大队路径。

## 标准流程

对每一个大队/路段档案目录执行：

```powershell
go run ./cmd/omap-import scan --root '档案目录' --segment xjk-g6-xiguojing --json
go run ./cmd/omap-import import --root '档案目录' --segment xjk-g6-xiguojing --json
```

先看 `scan` 输出中的 `warnings`、`total` 和 `items`。不能识别桩号的文件不得猜坐标，必须记录 warning 并在 manifest 中补齐。确认桩号、路段和分类无误后再执行 `import`。首次导入前可加 `--dry-run`。

增量依据是档案目录内的 `.qinghai-omap-import-state.json`。CLI 对图片内容计算 SHA-256；图片新增或内容变化时才会再次导入。不要删除这个状态文件，也不要手工修改 OMap `oobj.odb`。

## 档案命名和分组

文件名或父目录应包含 `G6 K1801+080`、`K1801+080` 或 `K1814` 这样的桩号。每一张图片文件就是一个点，图片之间不会合并，也不会生成 ZIP。Word、Excel、PDF、TXT 等非图片文件只可作为参考资料，不会被扫描或写入 OMap。需要把文字信息带入地图时，写到 manifest 的 `comment` 字段。

自动分类按目录和文件名关键词匹配，常见值包括：`桥梁`、`涵洞`、`隧道`、`收费站`、`服务区、停车区`、`车辆通道`、`行政许可`、`涉路施工监管`、`公路附属设施标志标牌`、`监控设施`、`ETC龙门架`、`情报板`、`高边坡`、`安全隐患`、`网格化联络表`、`劝返站点`、`跨线桥`。无法自动判断时使用 `manifest.json` 显式指定 `assetType`。

## 需要人工补充时使用 manifest.json

在档案根目录建立数组格式的 `manifest.json`：

```json
[
  {
    "path": "桥梁/大桥/001 K1807+228 天峻大桥.png",
    "station": "G6 K1807+228",
    "segmentId": "xjk-g6-xiguojing",
    "brigade": "韵家口大队",
    "assetType": "桥梁",
    "name": "天峻大桥",
    "comment": "桥梁独立文字说明、许可编号、巡查备注等。",
    "attachments": ["桥梁/大桥/001 K1807+228 天峻大桥.png"]
  }
]
```

`comment` 或单点命令的 `--comment` 会直接进入 OMap 点自身的备注字段，不会生成说明文档。点名最终会自动追加标准桩号，例如 `天峻大桥 G6 K1807+228`；桩号类型的点名严格是 `G6 K1807+228`。需要细分归档目录时用 `point --folder` 精确指定，导入结果的 `targetFolder` 可核对实际落点。

## 校验和交付

导入后重新打开 OMap，检查 `收藏夹 > 西宁高速支队 > 韵家口大队 > 路产类型`。随机抽查点位是否在主线、点是否可以直接拖动、图片是否能打开、备注是否出现在“备注”框。点位明显偏离、桩号超出所选路段覆盖范围或路段 ID 不确定时停止导入并报告，不得使用临近点代替。

在需要把变更带到另一台电脑时，不要复制完整 OMap。先在源电脑运行：

```powershell
go run ./cmd/omap-import delta --destination '增量包目录' --json
```

把生成的 `增量包目录\data` 和其中的状态文件复制到目标电脑，再将 `data` 内容合并到目标 OMap `data`。目标电脑必须先备份 `oobj.odb`，并确认 OMap 已退出。
