package locator

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type point struct {
	Lat float64
	Lon float64
}

type controlPoint struct {
	Meter    int
	Name     string
	RawMeter float64
}

type routeData struct {
	Code       string
	Name       string
	StartMeter int
	EndMeter   int
	Points     []point
	Controls   []controlPoint
}

type roadGeometry struct {
	OSMID   int64
	Highway string
	Ref     string
	Name    string
	Lanes   int
	OneWay  bool
	Points  []point
}

type Result struct {
	Input             string  `json:"input"`
	Route             string  `json:"route"`
	RouteName         string  `json:"routeName"`
	Station           string  `json:"station"`
	Meter             int     `json:"meter"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	CoordinateSystem  string  `json:"coordinateSystem"`
	Reference         string  `json:"reference"`
	NearestControl    string  `json:"nearestControl"`
	ControlDistanceM  int     `json:"controlDistanceM"`
	Coverage          string  `json:"coverage"`
	VerificationState string  `json:"verificationState"`
}

type Coverage struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Start      string `json:"start"`
	End        string `json:"end"`
	LengthKM   string `json:"lengthKm"`
	ControlQty int    `json:"controlQty"`
}

// MapPoint is a WGS-84 vertex from the embedded road geometry.
type MapPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// RoadFeature is one actual public road way near the resolved point. Main
// carriageways and motorway_link geometries are retained separately.
type RoadFeature struct {
	OSMID  int64      `json:"osmId"`
	Kind   string     `json:"kind"`
	Ref    string     `json:"ref"`
	Name   string     `json:"name"`
	Lanes  int        `json:"lanes"`
	OneWay bool       `json:"oneWay"`
	Points []MapPoint `json:"points"`
}

// LocalMap is a close, station-centred road scene rather than a route overview.
type LocalMap struct {
	Result          Result        `json:"result"`
	CenterLatitude  float64       `json:"centerLatitude"`
	CenterLongitude float64       `json:"centerLongitude"`
	RangeM          int           `json:"rangeM"`
	Roads           []RoadFeature `json:"roads"`
	MainlineOffsetM float64       `json:"mainlineOffsetM"`
	MainlineMatched bool          `json:"mainlineMatched"`
	MainlineWayQty  int           `json:"mainlineWayQty"`
	HeadingDegrees  float64       `json:"headingDegrees"`
	Snapshot        string        `json:"snapshot"`
}

// RouteHealth records the repeatable fit check between the calibrated station
// path and the independently embedded local road-network ways.
type RouteHealth struct {
	Code                string  `json:"code"`
	SampleQty           int     `json:"sampleQty"`
	ControlQty          int     `json:"controlQty"`
	MainlineWayQty      int     `json:"mainlineWayQty"`
	MaxMainlineOffsetM  float64 `json:"maxMainlineOffsetM"`
	MeanMainlineOffsetM float64 `json:"meanMainlineOffsetM"`
}

// NetworkHealth describes the embedded public network snapshot and its
// repeatable route-fit audit. It is intentionally data-oriented so future
// imagery, patrol, and survey sources can be added without changing callers.
type NetworkHealth struct {
	Snapshot    string        `json:"snapshot"`
	RoadWayQty  int           `json:"roadWayQty"`
	RampWayQty  int           `json:"rampWayQty"`
	VertexQty   int           `json:"vertexQty"`
	RouteAudits []RouteHealth `json:"routeAudits"`
}

const networkSnapshot = "OpenStreetMap 道路与匝道几何 · 2026-09-04"

var stationPattern = regexp.MustCompile(`(?i)^\s*(G6|S101)\s*[KＫ]?\s*(\d+)\s*(?:\+\s*(\d{1,3}))?\s*$`)

func Locate(input string) (Result, error) {
	routeCode, meter, err := parse(input)
	if err != nil {
		return Result{}, err
	}
	route, ok := routeByCode(routeCode)
	if !ok {
		return Result{}, fmt.Errorf("当前 Demo 未收录 %s", routeCode)
	}
	if meter < route.StartMeter || meter > route.EndMeter {
		return Result{}, fmt.Errorf("%s 仅覆盖 %s 至 %s", route.Code, formatStation(route.StartMeter), formatStation(route.EndMeter))
	}

	rawMeter := calibratedRawMeter(route.Controls, meter)
	p := pointAtDistance(route.Points, rawMeter)
	nearest, distance := nearestControl(route.Controls, meter)
	return Result{
		Input:             strings.TrimSpace(input),
		Route:             route.Code,
		RouteName:         route.Name,
		Station:           formatStation(meter),
		Meter:             meter,
		Latitude:          round(p.Lat, 7),
		Longitude:         round(p.Lon, 7),
		CoordinateSystem:  "WGS-84",
		Reference:         "OpenStreetMap 道路中心线，2026-09-04 下载；资料库桩号控制点校准",
		NearestControl:    nearest.Name,
		ControlDistanceM:  distance,
		Coverage:          fmt.Sprintf("%s 至 %s", formatStation(route.StartMeter), formatStation(route.EndMeter)),
		VerificationState: "公开路网快照与辖区控制点分段校准",
	}, nil
}

// LocateWithMap returns the station conversion and actual nearby OSM road ways
// together so the desktop view can stay focused on the selected road scene.
func LocateWithMap(input string) (LocalMap, error) {
	result, err := Locate(input)
	if err != nil {
		return LocalMap{}, err
	}
	const rangeM = 180.0
	const captureM = 460.0
	target := point{Lat: result.Latitude, Lon: result.Longitude}
	type candidate struct {
		road roadGeometry
		kind string
		dist float64
	}
	candidates := make([]candidate, 0, 64)
	for _, road := range embeddedRoadNetwork {
		dist := distanceToRoad(target, road.Points)
		if dist > captureM {
			continue
		}
		candidates = append(candidates, candidate{road: road, kind: roadKind(road, result.Route), dist: dist})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].kind != candidates[j].kind {
			return roadKindPriority(candidates[i].kind) < roadKindPriority(candidates[j].kind)
		}
		return candidates[i].dist < candidates[j].dist
	})
	if len(candidates) > 160 {
		candidates = candidates[:160]
	}
	roads := make([]RoadFeature, 0, len(candidates))
	mainlines := routeMainlines(result.Route)
	nearestMainlineM := nearestRoadDistance(target, mainlines)
	mainlineMatched := !math.IsInf(nearestMainlineM, 1)
	mainlineWayQty := 0
	for _, item := range candidates {
		if item.kind == "main" {
			mainlineWayQty++
		}
		points := make([]MapPoint, len(item.road.Points))
		for i, p := range item.road.Points {
			points[i] = MapPoint{Latitude: p.Lat, Longitude: p.Lon}
		}
		roads = append(roads, RoadFeature{
			OSMID: item.road.OSMID, Kind: item.kind, Ref: item.road.Ref, Name: item.road.Name,
			Lanes: item.road.Lanes, OneWay: item.road.OneWay, Points: points,
		})
	}
	if !mainlineMatched {
		nearestMainlineM = 0
	}
	route, _ := routeByCode(result.Route)
	return LocalMap{
		Result: result, CenterLatitude: result.Latitude, CenterLongitude: result.Longitude,
		RangeM: int(rangeM), Roads: roads, MainlineOffsetM: round(nearestMainlineM, 1), MainlineMatched: mainlineMatched,
		MainlineWayQty: mainlineWayQty, HeadingDegrees: headingAtDistance(route.Points, calibratedRawMeter(route.Controls, result.Meter)), Snapshot: networkSnapshot,
	}, nil
}

func CoverageList() []Coverage {
	coverage := make([]Coverage, 0, len(embeddedRoutes))
	for _, route := range embeddedRoutes {
		coverage = append(coverage, Coverage{
			Code: route.Code, Name: route.Name, Start: formatStation(route.StartMeter), End: formatStation(route.EndMeter),
			LengthKM: fmt.Sprintf("%.2f", float64(route.EndMeter-route.StartMeter)/1000), ControlQty: len(route.Controls),
		})
	}
	return coverage
}

// NetworkHealthReport runs a deterministic consistency check every 250 metres
// across the covered mainlines. It catches an accidental mismatch between the
// calibrated route path and the embedded public road snapshot after either is
// regenerated; it is not a field-survey accuracy assertion.
func NetworkHealthReport() NetworkHealth {
	health := NetworkHealth{Snapshot: networkSnapshot, RouteAudits: make([]RouteHealth, 0, len(embeddedRoutes))}
	for _, road := range embeddedRoadNetwork {
		health.RoadWayQty++
		health.VertexQty += len(road.Points)
		if road.Highway == "motorway_link" {
			health.RampWayQty++
		}
	}
	for _, route := range embeddedRoutes {
		mainlines := routeMainlines(route.Code)
		audit := RouteHealth{Code: route.Code, ControlQty: len(route.Controls), MainlineWayQty: len(mainlines)}
		var total float64
		for meter := route.StartMeter; ; meter += 250 {
			if meter > route.EndMeter {
				meter = route.EndMeter
			}
			target := pointAtDistance(route.Points, calibratedRawMeter(route.Controls, meter))
			nearest := nearestRoadDistance(target, mainlines)
			if math.IsInf(nearest, 1) {
				nearest = 0
			}
			audit.SampleQty++
			total += nearest
			if nearest > audit.MaxMainlineOffsetM {
				audit.MaxMainlineOffsetM = nearest
			}
			if meter == route.EndMeter {
				break
			}
		}
		if audit.SampleQty > 0 {
			audit.MeanMainlineOffsetM = round(total/float64(audit.SampleQty), 1)
		}
		audit.MaxMainlineOffsetM = round(audit.MaxMainlineOffsetM, 1)
		health.RouteAudits = append(health.RouteAudits, audit)
	}
	return health
}

func routeMainlines(route string) []roadGeometry {
	mainlines := make([]roadGeometry, 0, 64)
	for _, road := range embeddedRoadNetwork {
		if roadKind(road, route) == "main" {
			mainlines = append(mainlines, road)
		}
	}
	return mainlines
}

func nearestRoadDistance(target point, roads []roadGeometry) float64 {
	nearest := math.Inf(1)
	for _, road := range roads {
		if distance := distanceToRoad(target, road.Points); distance < nearest {
			nearest = distance
		}
	}
	return nearest
}

func roadKind(road roadGeometry, route string) string {
	if road.Highway == "motorway_link" {
		return "ramp"
	}
	for _, ref := range strings.Split(strings.ToUpper(road.Ref), ";") {
		if strings.TrimSpace(ref) == route {
			return "main"
		}
	}
	return "context"
}

func roadKindPriority(kind string) int {
	switch kind {
	case "main":
		return 0
	case "ramp":
		return 1
	default:
		return 2
	}
}

func distanceToRoad(target point, points []point) float64 {
	best := math.Inf(1)
	for i := 1; i < len(points); i++ {
		if d := distanceToSegment(target, points[i-1], points[i]); d < best {
			best = d
		}
	}
	return best
}

func distanceToSegment(target, a, b point) float64 {
	const degreesToMeters = 111320.0
	cosLatitude := math.Cos(target.Lat * math.Pi / 180)
	ax, ay := (a.Lon-target.Lon)*degreesToMeters*cosLatitude, (a.Lat-target.Lat)*degreesToMeters
	bx, by := (b.Lon-target.Lon)*degreesToMeters*cosLatitude, (b.Lat-target.Lat)*degreesToMeters
	dx, dy := bx-ax, by-ay
	denominator := dx*dx + dy*dy
	if denominator == 0 {
		return math.Hypot(ax, ay)
	}
	fraction := -(ax*dx + ay*dy) / denominator
	fraction = math.Max(0, math.Min(1, fraction))
	return math.Hypot(ax+fraction*dx, ay+fraction*dy)
}

func parse(input string) (string, int, error) {
	matches := stationPattern.FindStringSubmatch(strings.TrimSpace(input))
	if matches == nil {
		return "", 0, fmt.Errorf("请输入如 G6 K1792+200 或 S101 K12+700 的桩号")
	}
	km, err := strconv.Atoi(matches[2])
	if err != nil {
		return "", 0, fmt.Errorf("公里数无效")
	}
	meter := 0
	if matches[3] != "" {
		meter, err = strconv.Atoi(matches[3])
		if err != nil || meter > 999 {
			return "", 0, fmt.Errorf("加号后的米数应为 000 至 999")
		}
	}
	return strings.ToUpper(matches[1]), km*1000 + meter, nil
}

func routeByCode(code string) (routeData, bool) {
	for _, route := range embeddedRoutes {
		if route.Code == code {
			return route, true
		}
	}
	return routeData{}, false
}

func calibratedRawMeter(controls []controlPoint, meter int) float64 {
	idx := sort.Search(len(controls), func(i int) bool { return controls[i].Meter >= meter })
	if idx == 0 {
		return controls[0].RawMeter
	}
	if idx == len(controls) {
		return controls[len(controls)-1].RawMeter
	}
	a, b := controls[idx-1], controls[idx]
	fraction := float64(meter-a.Meter) / float64(b.Meter-a.Meter)
	return a.RawMeter + fraction*(b.RawMeter-a.RawMeter)
}

func nearestControl(controls []controlPoint, meter int) (controlPoint, int) {
	best := controls[0]
	for _, control := range controls[1:] {
		if abs(control.Meter-meter) < abs(best.Meter-meter) {
			best = control
		}
	}
	return best, abs(best.Meter - meter)
}

func pointAtDistance(points []point, target float64) point {
	var walked float64
	for i := 1; i < len(points); i++ {
		length := distance(points[i-1], points[i])
		if walked+length >= target {
			return interpolate(points[i-1], points[i], (target-walked)/length)
		}
		walked += length
	}
	return points[len(points)-1]
}

func headingAtDistance(points []point, target float64) float64 {
	var walked float64
	for i := 1; i < len(points); i++ {
		length := distance(points[i-1], points[i])
		if walked+length >= target {
			return round(initialBearing(points[i-1], points[i]), 1)
		}
		walked += length
	}
	return round(initialBearing(points[len(points)-2], points[len(points)-1]), 1)
}

func initialBearing(a, b point) float64 {
	lat1, lat2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}

func distance(a, b point) float64 {
	const earth = 6371008.8
	lat1, lat2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dLat, dLon := (b.Lat-a.Lat)*math.Pi/180, (b.Lon-a.Lon)*math.Pi/180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earth * 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}

func interpolate(a, b point, fraction float64) point {
	return point{Lat: a.Lat + (b.Lat-a.Lat)*fraction, Lon: a.Lon + (b.Lon-a.Lon)*fraction}
}
func formatStation(meter int) string { return fmt.Sprintf("K%d+%03d", meter/1000, meter%1000) }
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func round(value float64, decimals int) float64 {
	power := math.Pow10(decimals)
	return math.Round(value*power) / power
}
