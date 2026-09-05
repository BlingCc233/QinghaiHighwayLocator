# 青海高速路产定位

韵家口大队辖区的离线桩号转 WGS-84 经纬度、路产对象和奥维地图交换包桌面工具。

当前覆盖：

- G6 京藏高速：K1766+600 至 K1836+000
- S101 西宁至互助高速：K0+000 至 K41+430

## 使用

运行 `bin/qinghaihighwaylocator.exe`，先从线路下拉框选择 `G6` 或 `S101`，再在桩号框输入 `K1792+200` 或 `K12+700` 定位。界面会给出可复制的经纬度，并以当前桩号为中心显示约 360 m 的局部路网；缩放控件可在 190 至 680 m 场景范围间切换。

在左侧“路产点资料”填写类型和名称，并可拖入或通过原生文件选择器添加附件。首次使用或更换电脑时，点击“选择 data”指定该电脑的 OMAP `data` 文件夹。点击“写入奥维”后，程序会在奥维关闭时直接更新：

- `收藏夹 > 西宁高速支队 > 韵家口大队 > 路产类型` 下的新点对象
- `Documents\omap\data\attachment` 下的对象附件
- `Documents\omap\station_locator_backup` 下的写入前自动备份

选择“桩号”类型时，界面切换为批量模式，输入起止公里（例如 `1766` 到 `1836`），程序会按整公里生成 0 附件的桩号对象，名称自动形成为“点位名称 + 线路桩号”。不在当前已收录线路覆盖区间内的公里会明确跳过，不会写入虚构坐标。

奥维原生点记录只有一个稳定的附件引用槽位；选择多个附件时，程序会把所有文件打包到一个 ZIP 后作为该点的附件，文件内容不会丢失。写入完成后重新打开奥维即可看到目录和点位。

## 数据与实现

程序不要求养护或运营单位接口。桩号换算采用离线内置的道路中心线和辖区资料库控制点分段校准；控制点包括端点、收费站、互通及匝道等可识别位置。

局部地图使用 2026-09-04 下载的 OpenStreetMap `motorway` 与 `motorway_link` 原始几何，内置 1,235 条道路要素、8,808 个道路顶点。主线、分向道路和匝道从这些真实点列绘制，匝道不是前端示意短线。道路的 `lanes` 标签用来绘制车道分隔标记，Ping 点始终位于当前经纬度对应的位置。

数据署名：© OpenStreetMap contributors，遵循 [ODbL](https://www.openstreetmap.org/copyright)。公开数据快照和控制点均可版本化更新，后续可在此基础上接入影像复核、巡查记录和现场采集结果。

## 构建

```powershell
npm --prefix frontend install
npm --prefix frontend run build
go test ./...
wails3 build
```

Windows x64 产物位于 `bin/qinghaihighwaylocator.exe`。重新取得公开 OSM 快照后，在项目目录运行：

```powershell
go run .\tools\generate_network_data
```

然后重新构建。历史桩号换算的主线数据生成器保留在 `tools/generate_road_data.go`。

## Logo

给 Gemini 的 SVG Logo 提示词见 [`LOGO_PROMPT.md`](LOGO_PROMPT.md)。当前品牌标记从 `frontend/public/logo.svg` 加载；Windows 图标 `build/appicon.png` 与 `build/windows/icon.ico` 已同步使用该 SVG。
