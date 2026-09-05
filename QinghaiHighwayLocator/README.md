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

## Logo

给 Gemini 的 SVG Logo 提示词见 [`LOGO_PROMPT.md`](LOGO_PROMPT.md)。当前品牌标记从 `frontend/public/logo.svg` 加载；Windows 图标 `build/appicon.png` 与 `build/windows/icon.ico` 已同步使用该 SVG。
