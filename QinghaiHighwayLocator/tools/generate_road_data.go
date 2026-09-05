// Command generate_road_data converts downloaded OSM route geometry into the
// compact Go data embedded by the offline demo. It is intentionally not part of
// the application binary.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

type point struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type member struct {
	Type string `json:"type"`
	Ref  int64  `json:"ref"`
}

type element struct {
	Type     string   `json:"type"`
	ID       int64    `json:"id"`
	Geometry []point  `json:"geometry"`
	Members  []member `json:"members"`
}

type overpass struct {
	Elements []element `json:"elements"`
}

type landmark struct {
	Meter int
	Name  string
	Point point
}

type control struct {
	Meter    int
	Name     string
	RawMeter float64
	Point    point
	Offset   float64
}

func main() {
	g6 := buildPath("../g6-east-local.json", "../g6-east.json", 1543, 1731, func(a, b point) bool {
		return a.Lon < b.Lon // G6 mileage grows from the eastern end towards the west.
	})
	s101 := buildPath("../s101-local.json", "../s101.json", 0, 50, func(a, b point) bool {
		return a.Lat > b.Lat // S101 mileage grows from the G6 junction northwards.
	})

	g6Controls := calibrate(g6, []landmark{
		{1766600, "平安收费站", point{36.5091132, 102.0998693}},
		{1768800, "曹家堡东收费站", point{36.5191836, 102.0734066}},
		{1774300, "曹家堡收费站", point{36.5252860, 102.0287900}},
		// Mainline toll-plaza area; the earlier point was an off-ramp booth.
		{1779800, "海东主线收费站", point{36.5450486, 101.9670886}},
		{1790600, "韵家口立交", point{36.5838548, 101.8581891}},
		{1792400, "民和路出口", point{36.5891589, 101.8340859}},
		{1795000, "站东巷匝道", point{36.6186303, 101.8184177}},
		{1797700, "站西巷匝道", point{36.6244186, 101.8080439}},
		{1800500, "朝阳互通立交", point{36.6398609, 101.7821288}},
		{1816500, "西钢匝道", point{36.6810430, 101.6460794}},
		{1836000, "西宁西收费站", point{36.6591093, 101.4404679}},
	})

	s101Controls := calibrate(s101, []landmark{
		{0, "G6韵家口衔接处", point{36.5822891, 101.8569051}},
		{4500, "互助主线收费站", point{36.6125092, 101.8829240}},
		{15000, "塘川收费站", point{36.7016416, 101.9015787}},
		{27000, "互助南收费站", point{36.7944034, 101.9388470}},
		{32500, "互助东收费站", point{36.8122444, 102.0002560}},
		{41430, "互助端", s101[len(s101)-1]},
	})

	ensureMonotonic("G6", g6Controls)
	ensureMonotonic("S101", s101Controls)
	write("internal/locator/data_generated.go", g6, g6Controls, s101, s101Controls)
}

func buildPath(localFile, relationFile string, first, last int, reverseFirst func(point, point) bool) []point {
	local := read(localFile)
	relation := read(relationFile)
	ways := make(map[int64][]point)
	for _, e := range local.Elements {
		if e.Type == "way" && len(e.Geometry) > 1 {
			ways[e.ID] = e.Geometry
		}
	}
	var members []member
	for _, e := range relation.Elements {
		if e.Type == "relation" {
			members = e.Members
			break
		}
	}
	if len(members) == 0 || first < 0 || last >= len(members) {
		panic("route relation members are unavailable")
	}

	var path []point
	for i := first; i <= last; i++ {
		geom := append([]point(nil), ways[members[i].Ref]...)
		if len(geom) == 0 {
			panic(fmt.Sprintf("missing OSM way %d", members[i].Ref))
		}
		if len(path) == 0 {
			if reverseFirst(geom[0], geom[len(geom)-1]) {
				reverse(geom)
			}
		} else {
			end := path[len(path)-1]
			if distance(end, geom[len(geom)-1]) < distance(end, geom[0]) {
				reverse(geom)
			}
		}
		if len(path) > 0 && distance(path[len(path)-1], geom[0]) > 90 {
			panic(fmt.Sprintf("route discontinuity before member %d: %.1fm", i, distance(path[len(path)-1], geom[0])))
		}
		if len(path) > 0 {
			geom = geom[1:]
		}
		path = append(path, geom...)
	}
	return path
}

func calibrate(path []point, marks []landmark) []control {
	controls := make([]control, 0, len(marks))
	for _, mark := range marks {
		raw, snapped, offset := project(path, mark.Point)
		controls = append(controls, control{Meter: mark.Meter, Name: mark.Name, RawMeter: raw, Point: snapped, Offset: offset})
	}
	sort.Slice(controls, func(i, j int) bool { return controls[i].Meter < controls[j].Meter })
	for _, c := range controls {
		fmt.Fprintf(os.Stderr, "%s K%d+%03d raw=%.1fm landmark-offset=%.1fm\n", c.Name, c.Meter/1000, c.Meter%1000, c.RawMeter, c.Offset)
	}
	return controls
}

func ensureMonotonic(route string, controls []control) {
	for i := 1; i < len(controls); i++ {
		if controls[i].RawMeter <= controls[i-1].RawMeter {
			panic(fmt.Sprintf("%s controls are not monotonic: %s -> %s", route, controls[i-1].Name, controls[i].Name))
		}
	}
}

func project(path []point, target point) (float64, point, float64) {
	var bestAlong, bestOffset float64 = 0, math.Inf(1)
	var best point
	var walked float64
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		length := distance(a, b)
		if length == 0 {
			continue
		}
		t := projectedFraction(a, b, target)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		p := interpolate(a, b, t)
		offset := distance(p, target)
		if offset < bestOffset {
			bestAlong, best, bestOffset = walked+t*length, p, offset
		}
		walked += length
	}
	return bestAlong, best, bestOffset
}

func projectedFraction(a, b, p point) float64 {
	const degree = 111320.0
	cosLat := math.Cos((a.Lat + b.Lat + p.Lat) / 3 * math.Pi / 180)
	ax, ay := a.Lon*degree*cosLat, a.Lat*degree
	bx, by := b.Lon*degree*cosLat, b.Lat*degree
	px, py := p.Lon*degree*cosLat, p.Lat*degree
	dx, dy := bx-ax, by-ay
	return ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
}

func distance(a, b point) float64 {
	const earth = 6371008.8
	lat1, lat2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dLat, dLon := (b.Lat-a.Lat)*math.Pi/180, (b.Lon-a.Lon)*math.Pi/180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earth * 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}

func interpolate(a, b point, t float64) point {
	return point{Lat: a.Lat + (b.Lat-a.Lat)*t, Lon: a.Lon + (b.Lon-a.Lon)*t}
}
func reverse(points []point) {
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
}

func read(path string) overpass {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var data overpass
	if err := json.Unmarshal(b, &data); err != nil {
		panic(err)
	}
	return data
}

func write(file string, g6 []point, g6Controls []control, s101 []point, s101Controls []control) {
	var b strings.Builder
	b.WriteString("// Code generated by tools/generate_road_data.go; DO NOT EDIT.\n// Source: OpenStreetMap contributors (ODbL), downloaded 2026-09-04.\npackage locator\n\n")
	b.WriteString("var embeddedRoutes = []routeData{\n")
	writeRoute(&b, "G6", "京藏高速公路", 1766600, 1836000, g6, g6Controls)
	writeRoute(&b, "S101", "西宁至互助高速公路", 0, 41430, s101, s101Controls)
	b.WriteString("}\n")
	if err := os.WriteFile(file, []byte(b.String()), 0644); err != nil {
		panic(err)
	}
}

func writeRoute(b *strings.Builder, code, name string, start, end int, points []point, controls []control) {
	fmt.Fprintf(b, "{Code: %q, Name: %q, StartMeter: %d, EndMeter: %d, Points: []point{\n", code, name, start, end)
	for _, p := range points {
		fmt.Fprintf(b, "{Lat: %.7f, Lon: %.7f},\n", p.Lat, p.Lon)
	}
	b.WriteString("}, Controls: []controlPoint{\n")
	for _, c := range controls {
		fmt.Fprintf(b, "{Meter: %d, Name: %q, RawMeter: %.3f},\n", c.Meter, c.Name, c.RawMeter)
	}
	b.WriteString("}},\n")
}
