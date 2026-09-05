import { LocatorService } from "../bindings/stationnum2omap/index.js";
import type { Coverage, LocalMap, MapPoint, NetworkHealth, Result, RoadFeature, RouteSegment } from "../bindings/stationnum2omap/internal/locator/models.js";
import "../public/style.css";

const app = document.querySelector<HTMLDivElement>("#app")!;
const icon = {
  target: `<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="7"/><circle cx="12" cy="12" r="2"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3"/></svg>`,
  search: `<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="10.8" cy="10.8" r="5.8"/><path d="m16 16 4 4"/></svg>`,
  plus: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14"/></svg>`,
  minus: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"/></svg>`,
  locate: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m14.2 9.8-4.6 4.6m-3.3 2.8 1.7-4.5 8.9-6.1-6.1 8.9-4.5 1.7Z"/></svg>`,
  copy: `<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="8" y="8" width="11" height="11" rx="1"/><path d="M16 8V5.8A1.8 1.8 0 0 0 14.2 4H5.8A1.8 1.8 0 0 0 4 5.8v8.4A1.8 1.8 0 0 0 5.8 16H8"/></svg>`,
  file: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 3h8l4 4v14H6z"/><path d="M14 3v5h5M9 13h6M9 17h6"/></svg>`,
  export: `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v12m0 0 4-4m-4 4-4-4M5 19h14"/></svg>`,
};

app.innerHTML = `
<div class="app-frame">
  <header class="app-header">
    <div class="identity"><img class="identity-logo" src="/logo.svg" alt="青海高速路产定位" /><div><strong>青海高速路产定位</strong><span>韵家口大队 · 西宁高支路产对象台</span></div></div>
    <div class="header-status"><i></i><span>离线公开路网快照</span><b>WGS-84</b><time>2026.09.04</time></div>
  </header>
  <main class="map-stage" id="map-stage">
    <canvas id="road-map" aria-label="当前桩号附近的公开道路和匝道几何图"></canvas><div class="map-grain" aria-hidden="true"></div>
    <section class="station-console" aria-label="桩号定位">
      <div class="console-kicker"><span>STATION / POSITION</span><span class="live-mark"><i></i>LIVE</span></div><h1>定位一个桩号</h1><p>输入辖区道路里程，地图自动收束到当前位置。</p>
      <div class="jurisdiction-grid"><label for="brigade-select"><span>所属大队</span><select id="brigade-select"></select></label><label for="route-select"><span>线路 / 路段</span><select id="route-select"></select></label></div><form id="lookup-form"><label class="station-field" for="station-input"><span>${icon.search}</span><input id="station-input" value="K1792+200" autocomplete="off" spellcheck="false" placeholder="K1792+200" /></label><button class="locate-action" type="submit"><span>定位</span>${icon.target}</button></form>
      <div class="quick-stations" aria-label="常用桩号"><button type="button" data-route="G6" data-station="K1790+600">K1790+600</button><button type="button" data-route="G6" data-station="K1816+500">K1816+500</button><button type="button" data-route="S101" data-station="K12+700">S101 K12+700</button></div><p class="lookup-error" id="lookup-error" role="alert"></p>
      <div class="console-divider"></div><div class="scene-facts"><div><span>路网范围</span><strong id="map-range">360 m</strong></div><div><span>现场道路</span><strong id="road-count">--</strong></div><div><span>匝道几何</span><strong id="ramp-count">--</strong></div></div>
      <div class="asset-workbench"><div class="asset-heading"><span>OMAP OBJECT</span><strong>路产点资料</strong></div><div class="asset-grid"><label><span>路产类型</span><select id="asset-type"><option value="桥梁">桥梁</option><option value="隧道">隧道</option><option value="服务区">服务区</option><option value="收费站">收费站</option><option value="行政许可">行政许可</option><option value="桩号" selected>桩号</option></select></label><label id="asset-name-field"><span>点位名称</span><input id="asset-name" placeholder="例如：海东收费站" /></label></div><div class="mile-range" id="mile-range"><div><span>批量起始公里</span><input id="range-start" inputmode="numeric" placeholder="1766" /></div><div><span>批量结束公里</span><input id="range-end" inputmode="numeric" placeholder="1766" /></div><small>桩号模式按整公里生成 0 附件对象，名称严格为线路 + 完整桩号。</small></div><div class="attachment-drop" id="attachment-drop" tabindex="0"><span class="attachment-icon">${icon.file}</span><div><strong>拖入附件</strong><small id="attachment-summary">可选附件，写入后在奥维对象中查看</small></div><button type="button" id="pick-attachments" title="选择附件">选择</button></div><div class="attachment-list" id="attachment-list"></div><div class="omap-directory"><small id="export-directory">读取奥维 data 目录中</small><button type="button" id="pick-omap-data" title="选择奥维 data 文件夹">选择 data</button></div><div class="asset-actions"><small>对象将写入所选大队的收藏夹目录</small><button class="export-action" id="export-omap" type="button" title="写入奥维收藏夹">${icon.export}<span>写入奥维</span></button></div><p class="export-status" id="export-status" role="status">关闭奥维后可直接写入所选大队目录。</p></div>
    </section>
    <section class="coordinate-dock" aria-live="polite"><div class="dock-heading"><span>POSITION READOUT</span><strong id="result-station">G6 K1792+200</strong></div><div class="route-line"><span id="result-route">G6</span><small id="result-route-name">京藏高速青海段</small></div>
      <div class="coordinate-row"><div><span>纬度 / LAT</span><strong id="latitude">--</strong></div><button class="tool-button copy-button" data-copy="latitude" title="复制纬度" aria-label="复制纬度">${icon.copy}</button></div><div class="coordinate-row"><div><span>经度 / LON</span><strong id="longitude">--</strong></div><button class="tool-button copy-button" data-copy="longitude" title="复制经度" aria-label="复制经度">${icon.copy}</button></div>
      <div class="dock-meta"><div><span>最近控制点</span><strong id="nearest-control">--</strong></div><div><span>桩号差</span><strong id="control-distance">--</strong></div><div><span>快照主线差</span><strong id="mainline-offset">--</strong></div><div><span>路线走向</span><strong id="route-heading">--</strong></div></div><button class="reference-toggle" id="reference-toggle" type="button">数据依据 <span>+</span></button><p class="reference-detail" id="reference-detail"></p>
    </section>
    <div class="map-topline"><span class="map-orientation">N</span><span id="map-label">G6 · 当前桩号局部场景</span></div><div class="map-key"><span><i class="key-main"></i>主线</span><span><i class="key-ramp"></i>匝道</span><span><i class="key-lane"></i>车道标记</span></div>
    <div class="map-tools" aria-label="地图工具"><button class="tool-button" id="zoom-in" title="放大" aria-label="放大">${icon.plus}</button><button class="tool-button" id="recenter" title="回到桩号" aria-label="回到桩号">${icon.locate}</button><button class="tool-button" id="zoom-out" title="缩小" aria-label="缩小">${icon.minus}</button></div><div class="map-ping" id="map-ping" aria-label="当前桩号 Ping 点"><i></i><span></span></div><div class="map-scale"><i></i><span id="scale-label">50 m</span></div><div class="map-attribution">道路几何 © OpenStreetMap contributors · ODbL</div>
  </main>
  <section class="route-strip" aria-label="辖区路线"><div class="strip-intro"><span>NETWORK INDEX</span><strong>辖区路网</strong></div><div class="coverage-list" id="coverage-list"><span>读取中</span></div><button class="network-status" id="network-status" type="button" aria-expanded="false"><i></i><span><small>路网一致性检查</small><strong id="network-health-summary">正在检查</strong></span><b>+</b></button></section>
  <section class="network-drawer" id="network-drawer" aria-label="路网一致性检查"><div class="drawer-header"><div><span>NETWORK CHECK</span><strong>离线路网快照</strong></div><button class="drawer-close" id="drawer-close" type="button" aria-label="关闭路网检查">×</button></div><div class="network-totals"><div><span>道路要素</span><strong id="health-road-ways">--</strong></div><div><span>匝道要素</span><strong id="health-ramp-ways">--</strong></div><div><span>道路顶点</span><strong id="health-vertices">--</strong></div></div><div class="route-audits" id="route-audits"></div></section>
</div><div class="toast" id="toast" role="status"></div>`;

const brigadeSelect = document.querySelector<HTMLSelectElement>("#brigade-select")!;
const routeSelect = document.querySelector<HTMLSelectElement>("#route-select")!;
const input = document.querySelector<HTMLInputElement>("#station-input")!;
const form = document.querySelector<HTMLFormElement>("#lookup-form")!;
const canvas = document.querySelector<HTMLCanvasElement>("#road-map")!;
const stage = document.querySelector<HTMLElement>("#map-stage")!;
const context = canvas.getContext("2d")!;
const ping = document.querySelector<HTMLElement>("#map-ping")!;
const errorElement = document.querySelector<HTMLElement>("#lookup-error")!;
const toast = document.querySelector<HTMLElement>("#toast")!;
const assetType = document.querySelector<HTMLSelectElement>("#asset-type")!;
const assetName = document.querySelector<HTMLInputElement>("#asset-name")!;
const assetNameField = document.querySelector<HTMLElement>("#asset-name-field")!;
const rangePanel = document.querySelector<HTMLElement>("#mile-range")!;
const rangeStart = document.querySelector<HTMLInputElement>("#range-start")!;
const rangeEnd = document.querySelector<HTMLInputElement>("#range-end")!;
const attachmentDrop = document.querySelector<HTMLElement>("#attachment-drop")!;
const attachmentList = document.querySelector<HTMLElement>("#attachment-list")!;
const attachmentSummary = document.querySelector<HTMLElement>("#attachment-summary")!;
const exportStatus = document.querySelector<HTMLElement>("#export-status")!;
const exportDirectory = document.querySelector<HTMLElement>("#export-directory")!;
const exportButton = document.querySelector<HTMLButtonElement>("#export-omap")!;
const pickOmapDataButton = document.querySelector<HTMLButtonElement>("#pick-omap-data")!;
const pickedAttachments: string[] = [];
let omapDataDirectory = localStorage.getItem("qinghai-omap-data-directory") ?? "";
type ScreenPoint = { x: number; y: number };
let activeScene: LocalMap | null = null;
let rangeM = 180;
let toastTimer: ReturnType<typeof setTimeout> | undefined;

type OmapPointInput = { station: string; brigade: string; segmentId: string; assetType: string; name: string; attachments: string[]; outputDirectory?: string; syncToOmap: boolean };
type OmapRangeInput = { route: string; brigade: string; segmentId: string; startStation: string; endStation: string; name?: string; outputDirectory?: string };
type OmapExportResult = { targetFolder: string; objectId: number; attachmentQty: number; omapAttachmentQty: number; dataFile?: string; backupDirectory?: string };
type OmapRangeResult = { targetFolder: string; objectIds: number[]; firstStation: string; lastStation: string; written: number; skipped: number };
const omapService = LocatorService as typeof LocatorService & { PickAttachments(): Promise<string[]>; PickOmapDataDirectory(): Promise<string>; GetOmapExportDirectory(): Promise<string>; GetRouteCatalog(): Promise<RouteSegment[]>; ExportOmap(input: OmapPointInput): Promise<OmapExportResult>; ExportOmapRange(input: OmapRangeInput): Promise<OmapRangeResult> };
let routeCatalog: RouteSegment[] = [];
let routeCatalogReady: Promise<void> = Promise.resolve();

function selectedSegment(): RouteSegment | undefined {
  return routeCatalog.find((item) => item.id === routeSelect.value) ?? routeCatalog[0];
}

function applySegmentDefaults(segment = selectedSegment()) {
  if (!segment) return;
  // Batch import accepts whole kilometres. Round inward so generated
  // stations stay inside the selected segment's exact metre range.
  rangeStart.value = String(Math.ceil(segment.startMeter / 1000));
  rangeEnd.value = String(Math.floor(segment.endMeter / 1000));
}

function renderRouteSegments() {
  const items = routeCatalog.filter((item) => item.brigade === brigadeSelect.value);
  const current = routeSelect.value;
  routeSelect.innerHTML = items.map((item) => "<option value=\"" + item.id + "\">" + item.code + " · " + item.segmentName + " · " + item.start + "-" + item.end + "</option>").join("");
  if (items.some((item) => item.id === current)) routeSelect.value = current;
  applySegmentDefaults();
}

function renderRouteCatalog(items: RouteSegment[]) {
  routeCatalog = items ?? [];
  brigadeSelect.innerHTML = [...new Set(routeCatalog.map((item) => item.brigade))].map((item) => "<option value=\"" + item + "\">" + item + "</option>").join("");
  renderRouteSegments();
}

function composeStation() {
  const raw = input.value.trim().replace(/^(G6|G0611|G0612|G569|G341|S101|S104)\s*/i, "");
  return (selectedSegment()?.code ?? "G6") + " " + raw;
}

function setRouteAndStation(value: string, route?: string) {
  const station = value.trim().replace(/^(G6|G0611|G0612|G569|G341|S101|S104)\s*/i, "");
  if (route) {
    const stationMatch = station.match(/^K?\s*(\d+)\s*(?:\+\s*(\d{1,3}))?$/i);
    const meter = stationMatch ? Number(stationMatch[1]) * 1000 + Number(stationMatch[2] ?? 0) : -1;
    const match = routeCatalog.find((item) => item.code === route && meter >= item.startMeter && meter <= item.endMeter)
      ?? routeCatalog.find((item) => item.code === route);
    if (match) {
      brigadeSelect.value = match.brigade;
      renderRouteSegments();
      routeSelect.value = match.id;
      applySegmentDefaults(match);
    }
  }
  input.value = station;
}

function updateAssetMode() {
  const batch = assetType.value === "桩号";
  rangePanel.hidden = !batch;
  attachmentDrop.hidden = batch;
  attachmentList.hidden = batch;
  assetNameField.hidden = batch;
  assetName.disabled = batch;
  if (batch) attachmentSummary.textContent = "批量桩号固定为 0 附件";
  else renderAttachments();
}

function showToast(message: string) { toast.textContent = message; toast.classList.add("is-visible"); if (toastTimer) window.clearTimeout(toastTimer); toastTimer = window.setTimeout(() => toast.classList.remove("is-visible"), 1800); }
function color(name: string) { return getComputedStyle(document.documentElement).getPropertyValue(name).trim(); }
function project(point: MapPoint, width: number, height: number): ScreenPoint { const scene = activeScene!; const meters = 111320; const dx = (point.longitude - scene.centerLongitude) * meters * Math.cos((scene.centerLatitude * Math.PI) / 180); const dy = (point.latitude - scene.centerLatitude) * meters; const scale = Math.min(width, height) / (rangeM * 2); return { x: width / 2 + dx * scale, y: height / 2 - dy * scale }; }
function drawPath(points: ScreenPoint[], width: number, stroke: string, dash: number[] = []) { if (points.length < 2) return; context.save(); context.beginPath(); context.moveTo(points[0].x, points[0].y); points.slice(1).forEach((point) => context.lineTo(point.x, point.y)); context.lineCap = "round"; context.lineJoin = "round"; context.lineWidth = width; context.strokeStyle = stroke; context.setLineDash(dash); context.stroke(); context.restore(); }
function offsetPolyline(points: ScreenPoint[], offset: number): ScreenPoint[] { return points.map((point, index) => { const previous = points[Math.max(0, index - 1)]; const next = points[Math.min(points.length - 1, index + 1)]; const dx = next.x - previous.x; const dy = next.y - previous.y; const length = Math.hypot(dx, dy) || 1; return { x: point.x - (dy / length) * offset, y: point.y + (dx / length) * offset }; }); }
function isVisible(points: ScreenPoint[], width: number, height: number) { return points.some((point) => point.x >= -90 && point.x <= width + 90 && point.y >= -90 && point.y <= height + 90); }
function paintRoad(road: RoadFeature, width: number, height: number) { const points = (road.points ?? []).map((point) => project(point, width, height)); if (!isVisible(points, width, height)) return; if (road.kind === "ramp") { drawPath(points, 12, color("--road-shadow")); drawPath(points, 8, color("--ramp")); drawPath(points, 1, color("--ramp-edge")); return; } if (road.kind === "context") { drawPath(points, 10, color("--road-shadow")); drawPath(points, 6, color("--context-road")); return; } const roadWidth = Math.max(13, Math.min(27, road.lanes * 7 + 8)); drawPath(points, roadWidth + 6, color("--road-shadow")); drawPath(points, roadWidth, color("--main-road")); drawPath(offsetPolyline(points, roadWidth / 2 - 1), 1.1, color("--road-edge")); drawPath(offsetPolyline(points, -roadWidth / 2 + 1), 1.1, color("--road-edge")); if (road.lanes > 1) for (let lane = 1; lane < road.lanes; lane += 1) drawPath(offsetPolyline(points, -roadWidth / 2 + (roadWidth * lane) / road.lanes), .75, color("--lane-line"), [7, 8]); }
function drawGrid(width: number, height: number) { context.save(); context.strokeStyle = color("--map-grid"); context.lineWidth = 1; context.setLineDash([1, 9]); for (let x = 0; x < width; x += 56) { context.beginPath(); context.moveTo(x, 0); context.lineTo(x, height); context.stroke(); } for (let y = 0; y < height; y += 56) { context.beginPath(); context.moveTo(0, y); context.lineTo(width, y); context.stroke(); } context.restore(); }
function drawMap() { const bounds = stage.getBoundingClientRect(); const width = Math.max(1, bounds.width); const height = Math.max(1, bounds.height); const ratio = window.devicePixelRatio || 1; canvas.width = Math.round(width * ratio); canvas.height = Math.round(height * ratio); canvas.style.width = `${width}px`; canvas.style.height = `${height}px`; context.setTransform(ratio, 0, 0, ratio, 0, 0); context.fillStyle = color("--map-bg"); context.fillRect(0, 0, width, height); drawGrid(width, height); if (!activeScene) { ping.hidden = true; return; } const roads = activeScene.roads ?? []; roads.filter((road) => road.kind === "context").forEach((road) => paintRoad(road, width, height)); roads.filter((road) => road.kind === "ramp").forEach((road) => paintRoad(road, width, height)); roads.filter((road) => road.kind === "main").forEach((road) => paintRoad(road, width, height)); const point = project({ latitude: activeScene.result.latitude, longitude: activeScene.result.longitude }, width, height); ping.hidden = false; ping.style.left = `${point.x}px`; ping.style.top = `${point.y}px`; const scaleM = rangeM <= 125 ? 25 : rangeM <= 190 ? 50 : 100; const scalePx = (scaleM / (rangeM * 2)) * Math.min(width, height); document.querySelector<HTMLElement>(".map-scale i")!.style.width = `${Math.max(30, scalePx)}px`; document.querySelector("#scale-label")!.textContent = `${scaleM} m`; }
function compassName(degrees: number) { const labels = ["北", "东北", "东", "东南", "南", "西南", "西", "西北"]; return labels[Math.round(degrees / 45) % 8]; }
function setScene(scene: LocalMap) { activeScene = scene; rangeM = scene.rangeM; const result: Result = scene.result; const roads = scene.roads ?? []; document.querySelector("#result-station")!.textContent = `${result.route} ${result.station}`; document.querySelector("#result-route")!.textContent = result.route; document.querySelector("#result-route-name")!.textContent = result.routeName; document.querySelector("#latitude")!.textContent = result.latitude.toFixed(7); document.querySelector("#longitude")!.textContent = result.longitude.toFixed(7); document.querySelector("#nearest-control")!.textContent = result.nearestControl; document.querySelector("#control-distance")!.textContent = `${result.controlDistanceM.toLocaleString("zh-CN")} m`; document.querySelector("#mainline-offset")!.textContent = scene.mainlineMatched ? `${scene.mainlineOffsetM.toFixed(1)} m` : "未匹配"; document.querySelector("#route-heading")!.textContent = `${compassName(scene.headingDegrees)} ${scene.headingDegrees.toFixed(0)}°`; document.querySelector("#reference-detail")!.textContent = result.reference; document.querySelector("#map-label")!.textContent = `${result.route} ${result.station} · 局部道路场景`; document.querySelector("#map-range")!.textContent = `${rangeM * 2} m`; document.querySelector("#road-count")!.textContent = `${roads.filter((road) => road.kind === "main").length} 段`; document.querySelector("#ramp-count")!.textContent = `${roads.filter((road) => road.kind === "ramp").length} 段`; stage.classList.add("is-ready"); drawMap(); }
async function locate() { await routeCatalogReady; const station = composeStation(); errorElement.textContent = ""; if (!input.value.trim()) { errorElement.textContent = "请输入桩号"; input.focus(); return; } const segment = selectedSegment(); if (!segment) { errorElement.textContent = "请选择辖区路段"; return; } form.classList.add("is-loading"); try { setScene(await omapService.LocateWithMapForSegment(segment.id, station)); } catch (error) { errorElement.textContent = error instanceof Error ? error.message : String(error); } finally { form.classList.remove("is-loading"); } }
function zoom(direction: 1 | -1) { const levels = [95, 130, 180, 250, 340]; const current = levels.reduce((best, value) => Math.abs(value - rangeM) < Math.abs(best - rangeM) ? value : best, levels[0]); const index = Math.max(0, Math.min(levels.length - 1, levels.indexOf(current) - direction)); rangeM = levels[index]; document.querySelector("#map-range")!.textContent = `${rangeM * 2} m`; drawMap(); }
function renderCoverage(items: Coverage[] | null) { const list = document.querySelector<HTMLElement>("#coverage-list")!; list.innerHTML = (items ?? []).map((item) => `<button class="coverage-item" type="button" data-route="${item.code}" data-station="${item.start}" data-segment="${item.segmentId ?? ""}"><span class="coverage-code ${item.code.toLowerCase()}">${item.code}</span><strong>${item.brigade ?? ""} · ${item.segmentName ?? item.name}</strong><small>${item.start} - ${item.end}</small></button>`).join(""); list.querySelectorAll<HTMLButtonElement>("[data-station]").forEach((button) => button.addEventListener("click", () => { const segmentID = button.dataset.segment; if (segmentID) { const segment = routeCatalog.find((item) => item.id === segmentID); if (segment) { brigadeSelect.value = segment.brigade; renderRouteSegments(); routeSelect.value = segment.id; } input.value = (button.dataset.station ?? "").replace(/^(G6|G0611|G0612|G569|G341|S101|S104)\s*/i, ""); } else { setRouteAndStation(button.dataset.station ?? "", button.dataset.route); } void locate(); })); }
function renderNetworkHealth(health: NetworkHealth) { document.querySelector("#network-health-summary")!.textContent = `${health.routeAudits?.length ?? 0} 条路线 · 一致性已检查`; document.querySelector("#health-road-ways")!.textContent = health.roadWayQty.toLocaleString("zh-CN"); document.querySelector("#health-ramp-ways")!.textContent = health.rampWayQty.toLocaleString("zh-CN"); document.querySelector("#health-vertices")!.textContent = health.vertexQty.toLocaleString("zh-CN"); document.querySelector("#route-audits")!.innerHTML = (health.routeAudits ?? []).map((audit) => `<article class="route-audit"><div><strong>${audit.code}</strong><span>${audit.sampleQty} 个里程样本 · ${audit.controlQty} 个控制点</span></div><div><small>快照均值差</small><b>${audit.meanMainlineOffsetM.toFixed(1)} m</b></div><div><small>快照最大差</small><b>${audit.maxMainlineOffsetM.toFixed(1)} m</b></div></article>`).join(""); }
function renderAttachments() { attachmentList.innerHTML = pickedAttachments.map((path, index) => `<div class="attachment-chip"><span>${icon.file}</span><strong title="${path}">${path.split(/[\\/]/).pop() ?? path}</strong><button type="button" data-remove-attachment="${index}" aria-label="移除附件">×</button></div>`).join(""); attachmentSummary.textContent = pickedAttachments.length ? `${pickedAttachments.length} 个附件已加入对象` : "也可点击选择，支持多个文件"; attachmentList.querySelectorAll<HTMLButtonElement>("[data-remove-attachment]").forEach((button) => button.addEventListener("click", () => { pickedAttachments.splice(Number(button.dataset.removeAttachment), 1); renderAttachments(); })); }
async function pickAttachments() { try { const paths = await omapService.PickAttachments(); for (const path of paths ?? []) if (!pickedAttachments.includes(path)) pickedAttachments.push(path); renderAttachments(); } catch (error) { showToast(error instanceof Error ? error.message : String(error)); } }
async function exportOmap() {
  const name = assetName.value.trim();
  if (assetType.value !== "桩号" && !name) { exportStatus.textContent = "请输入路产或 Ping 点名称。"; assetName.focus(); return; }
  if (assetType.value !== "桩号" && !activeScene) { exportStatus.textContent = "请先完成桩号定位。"; return; }
  exportButton.disabled = true;
  exportButton.classList.add("is-loading");
  exportStatus.textContent = "正在备份并写入奥维 data，请保持奥维完全关闭。";
  try {
    if (assetType.value === "桩号") {
      if (!rangeStart.value.trim() || !rangeEnd.value.trim()) { exportStatus.textContent = "请输入批量起止公里。"; rangeStart.focus(); return; }
      const segment = selectedSegment();
      if (!segment) { exportStatus.textContent = "请选择辖区路段。"; return; }
      const result = await omapService.ExportOmapRange({ route: segment.code, brigade: segment.brigade, segmentId: segment.id, startStation: rangeStart.value.trim(), endStation: rangeEnd.value.trim(), outputDirectory: omapDataDirectory });
      const skippedText = result.skipped ? `，覆盖外跳过 ${result.skipped} 个` : "";
      exportStatus.textContent = `已写入 ${result.targetFolder}，${result.written} 个整公里点（${result.firstStation} - ${result.lastStation}）${skippedText}。现在打开奥维即可查看。`;
      pickedAttachments.splice(0, pickedAttachments.length);
      renderAttachments();
    } else {
      const segment = selectedSegment();
      if (!segment) { exportStatus.textContent = "请选择辖区路段。"; return; }
      const result = await omapService.ExportOmap({ station: composeStation(), brigade: segment.brigade, segmentId: segment.id, assetType: assetType.value, name, attachments: pickedAttachments, outputDirectory: omapDataDirectory, syncToOmap: true });
      const attachmentText = result.attachmentQty
        ? `，附件 ${result.attachmentQty} 个${result.attachmentQty > 1 ? "（已打包为一个奥维附件）" : ""}`
        : "";
      exportStatus.textContent = `已写入 ${result.targetFolder}，对象 #${result.objectId}${attachmentText}。现在打开奥维即可查看。`;
      pickedAttachments.splice(0, pickedAttachments.length);
      renderAttachments();
    }
    showToast("已写入奥维收藏夹");
  } catch (error) {
    exportStatus.textContent = error instanceof Error ? error.message : String(error);
    showToast("奥维写入失败，原文件已保留");
  } finally {
    exportButton.disabled = false;
    exportButton.classList.remove("is-loading");
  }
}
function toggleNetworkDrawer(force?: boolean) { const drawer = document.querySelector("#network-drawer")!; const status = document.querySelector<HTMLButtonElement>("#network-status")!; const open = force ?? !drawer.classList.contains("is-open"); drawer.classList.toggle("is-open", open); status.setAttribute("aria-expanded", String(open)); }
brigadeSelect.addEventListener("change", () => { renderRouteSegments(); const segment = selectedSegment(); if (segment) input.value = segment.start; void locate(); });
routeSelect.addEventListener("change", () => { const segment = selectedSegment(); if (segment) { input.value = segment.start; applySegmentDefaults(segment); } void locate(); });
routeCatalogReady = omapService.GetRouteCatalog().then((items) => { renderRouteCatalog(items); renderCoverage(items as unknown as Coverage[]); });
form.addEventListener("submit", (event) => { event.preventDefault(); void locate(); }); document.querySelectorAll<HTMLButtonElement>(".quick-stations button").forEach((button) => button.addEventListener("click", () => { setRouteAndStation(button.dataset.station ?? "", button.dataset.route); void locate(); })); document.querySelectorAll<HTMLButtonElement>(".copy-button").forEach((button) => button.addEventListener("click", async () => { const value = document.querySelector<HTMLElement>(`#${button.dataset.copy}`)?.textContent ?? ""; if (!value || value === "--") return; try { await navigator.clipboard.writeText(value); showToast("坐标已复制"); } catch { showToast("复制失败"); } })); document.querySelector("#zoom-in")!.addEventListener("click", () => zoom(1)); document.querySelector("#zoom-out")!.addEventListener("click", () => zoom(-1)); document.querySelector("#recenter")!.addEventListener("click", () => { if (activeScene) { rangeM = activeScene.rangeM; drawMap(); } }); document.querySelector("#reference-toggle")!.addEventListener("click", () => document.querySelector(".coordinate-dock")!.classList.toggle("show-reference")); document.querySelector("#network-status")!.addEventListener("click", () => toggleNetworkDrawer()); document.querySelector("#drawer-close")!.addEventListener("click", () => toggleNetworkDrawer(false)); document.querySelector("#pick-attachments")!.addEventListener("click", () => void pickAttachments()); pickOmapDataButton.addEventListener("click", async () => { try { const directory = await omapService.PickOmapDataDirectory(); if (directory) { omapDataDirectory = directory; localStorage.setItem("qinghai-omap-data-directory", directory); exportDirectory.textContent = `已选择：${directory}`; showToast("已选择奥维 data 目录"); } } catch (error) { showToast(error instanceof Error ? error.message : String(error)); } }); assetType.addEventListener("change", updateAssetMode); attachmentDrop.addEventListener("click", (event) => { if ((event.target as HTMLElement).closest("button")) return; void pickAttachments(); }); attachmentDrop.addEventListener("keydown", (event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); void pickAttachments(); } }); ["dragenter", "dragover"].forEach((eventName) => attachmentDrop.addEventListener(eventName, (event) => { event.preventDefault(); attachmentDrop.classList.add("is-dragging"); })); ["dragleave", "drop"].forEach((eventName) => attachmentDrop.addEventListener(eventName, (event) => { event.preventDefault(); attachmentDrop.classList.remove("is-dragging"); })); attachmentDrop.addEventListener("drop", (event) => { const files = [...(event as DragEvent).dataTransfer?.files ?? []]; for (const file of files) { const path = (file as File & { path?: string }).path; if (path && !pickedAttachments.includes(path)) pickedAttachments.push(path); } renderAttachments(); }); exportButton.addEventListener("click", () => void exportOmap()); window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", drawMap); window.addEventListener("resize", drawMap); void LocatorService.GetCoverage().then(renderCoverage).catch(() => { document.querySelector("#coverage-list")!.textContent = "辖区路线加载失败"; }); void LocatorService.GetNetworkHealth().then(renderNetworkHealth).catch(() => { document.querySelector("#network-health-summary")!.textContent = "审计读取失败"; }); void omapService.GetOmapExportDirectory().then((directory) => { if (!omapDataDirectory) { omapDataDirectory = directory; localStorage.setItem("qinghai-omap-data-directory", directory); } exportDirectory.textContent = `已选择：${omapDataDirectory}`; }).catch(() => { exportDirectory.textContent = omapDataDirectory || "请选择奥维 data 文件夹"; }); updateAssetMode(); void locate();
