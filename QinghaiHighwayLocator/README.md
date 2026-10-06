# 青海高速路产定位

西宁高速支队多大队离线桩号转 WGS-84 经纬度、路产对象和奥维地图原生点写入工具。

线路目录按大队和路段组织，当前包含：韵家口（G6 平安—西宁、G6 西过境、S101）、大通（G0611 西宁—大通、G0611 大通主线、G569）、南绕城（G0612 主线及东延）、湟中（S104 西宁—湟中、S104 湟中—贵德）、湟源（G6 西宁—湟源、G6 湟源—倒淌河）、大通城关（G341）。每段均有明确的起止桩号。

## 使用

运行 `bin/qinghaihighwaylocator.exe`，先选择所属大队和线路/路段，再在桩号框输入 `K1792+200` 或 `K12+700` 定位。界面会给出可复制的经纬度，并以当前桩号为中心显示约 360 m 的局部路网；缩放控件可在 190 至 680 m 场景范围间切换。

在左侧“路产点资料”填写类型和名称，并可拖入或通过原生文件选择器添加附件。首次使用或更换电脑时，点击“选择 data”指定该电脑的 OMAP `data` 文件夹。点击“写入奥维”后，程序会在奥维关闭时直接更新：

- `收藏夹 > 西宁高速支队 > 所选大队 > 路产类型` 下的新点对象
- `Documents\omap\data\attachment` 下的对象附件
- `Documents\omap\station_locator_backup` 下的写入前自动备份

选择“桩号”类型时，界面切换为批量模式，输入起止公里（例如 `1766` 到 `1836`），程序会按整公里生成 0 附件对象，名称严格为“线路 + 完整桩号”（例如 `G6 K1792+000`），不需要点位名称。不在所选路段覆盖区间内的公里会明确跳过，不会写入虚构坐标。

奥维原生点记录只有一个稳定的附件引用槽位；选择多个附件时，程序会把所有文件打包到一个 ZIP 后作为该点的附件，文件内容不会丢失。写入完成后重新打开奥维即可看到目录和点位。

## 数据与实现

程序不要求养护或运营单位接口。桩号换算采用离线内置的道路中心线、公开可核对的 OSM 设施点和辖区提供的收费站/互通控制点分段校准；没有完整官方里程线位的路段使用端点和控制点推算并吸附公开主线，界面会明确标注“建议现场复核”。

局部地图使用 2026-09-04/09-05 下载的 OpenStreetMap `motorway`、`motorway_link`、`trunk`、`trunk_link`、`primary`、`primary_link` 原始几何。主线、分向道路和匝道从真实点列绘制，匝道不是前端示意短线；道路的 `lanes` 标签用来绘制车道分隔标记，Ping 点始终位于当前经纬度对应的位置。

韵家口大队三段路线另沿现有桩号换算线每 100 米采样一次 [SRTM 30 m 地形高程](https://www.opentopodata.org/datasets/srtm/)，末端不足 100 米另取终点。2026-09-28 的离线剖面共 1112 个样本；按桩号长度做梯形积分，G6 平安—西宁段平均约 2172.4 米、G6 西过境段约 2351.0 米、S101 段约 2422.3 米，三段合计 110.83 公里的里程加权平均约 2323.0 米。写入韵家口点位时按相邻样本插值得到该桩号的估算海拔，不使用辖区平均值替代单点高度。数据可通过 `go run ./tools/generate_elevation_data` 重新生成。

这些数值是地形高程估算，不能等同测量所得的路面海拔；桥面、隧道和高架路段尤其可能偏离 DEM。奥维三维地形模型与 SRTM 数据也可能存在高程基准和分辨率差异，需用奥维手动标点及现场高程对照核验贴地效果。

数据署名：© OpenStreetMap contributors，遵循 [ODbL](https://www.openstreetmap.org/copyright)。公开数据快照和控制点均可版本化更新，后续可在此基础上接入影像复核、巡查记录和现场采集结果。

## 构建

```powershell
npm --prefix frontend install
npm --prefix frontend run build
go test ./...
wails3 build
```

Windows x64 产物位于 `bin/qinghaihighwaylocator.exe`。构建时会用 `build/windows/icon.ico` 生成原生图标资源，并嵌入 `build/windows/wails.exe.manifest`；当前清单设置为 `requireAdministrator`，启动和写入受保护的 OMAP `data` 目录时会显示 UAC 提示。重新取得公开 OSM 快照后，在项目目录运行：

```powershell
go run .\tools\generate_network_data
```

然后重新构建。历史桩号换算的主线数据生成器保留在 `tools/generate_road_data.go`。

## CLI 批量导入与增量同步

项目还提供不依赖桌面界面的 `cmd/omap-import`。先关闭 OMap，并设置其 `data` 目录：

```powershell
$env:QINGHAI_OMAP_DATA = 'C:\Users\13421\Documents\omap\data'
go run ./cmd/omap-import scan --root '档案目录' --segment xjk-g6-xiguojing --json
go run ./cmd/omap-import import --root '档案目录' --segment xjk-g6-xiguojing --json
```

CLI 从文件名/父目录识别 `G6 K1801+080`、`K1801+080` 或 `K1814` 桩号。扫描只收录图片文件（jpg/jpeg/png/webp/bmp/gif/tif/tiff），每张图片建立一个独立点，绝不生成 ZIP、DOC 或其它打包附件；`manifest.json` 可显式指定 `segmentId`、`assetType`、`name`、`comment` 和图片路径。`comment` 会写入 OMap 点自身的备注字段。每个新点都写入可编辑元数据，导入后可直接拖动。状态保存在档案根目录的 `.qinghai-omap-import-state.json`，只会处理新增或内容变化的图片。

单独录入一个点时，可指定大队下的精确目录、名称、图片、备注、桩号和 OMap data 路径：

```powershell
go run ./cmd/omap-import point --segment xjk-g6-xiguojing --station 'G6 K1807+228' --folder '行政许可/跨越公路' --name '跨越公路架设电缆' --comment '许可编号：青交许字〔2026〕1号' --attachment '.\57+060.jpg' --data 'C:\Users\13421\Documents\omap\data' --json
go run ./cmd/omap-import help point
```

`--folder` 从路产类型开始，归档在 `收藏夹 > 西宁高速支队 > 韵家口大队` 下；`--data` 覆盖环境变量。所有子命令支持 `-h`、`--help`，顶层 `-h` 列出命令。

查看变更状态：

```powershell
go run ./cmd/omap-import status --root '档案目录' --json
```

只带走 OMap 变更文件：

```powershell
go run ./cmd/omap-import delta --destination '增量包目录' --json
```

给其它 AI agent 的完整操作约束见 [`docs/AI_AGENT_OMAP_IMPORT.md`](docs/AI_AGENT_OMAP_IMPORT.md)。

## Logo

给 Gemini 的 SVG Logo 提示词见 [`LOGO_PROMPT.md`](LOGO_PROMPT.md)。当前品牌标记从 `frontend/public/logo.svg` 加载；Windows 图标 `build/appicon.png` 与 `build/windows/icon.ico` 已同步使用该 SVG。
