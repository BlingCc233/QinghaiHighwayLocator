package locator

import (
	"strings"
	"testing"
)

func TestLocateKnownRoutes(t *testing.T) {
	tests := []struct {
		input          string
		route          string
		minLat, maxLat float64
		minLon, maxLon float64
	}{
		{"G6 K1792+200", "G6", 36.4, 36.8, 101.4, 102.2},
		{"S101 K12+700", "S101", 36.5, 36.9, 101.7, 102.1},
	}
	for _, tt := range tests {
		result, err := Locate(tt.input)
		if err != nil {
			t.Fatalf("Locate(%q) returned error: %v", tt.input, err)
		}
		if result.Route != tt.route || result.Station == "" {
			t.Errorf("Locate(%q) returned route/station %q/%q", tt.input, result.Route, result.Station)
		}
		if result.Latitude < tt.minLat || result.Latitude > tt.maxLat || result.Longitude < tt.minLon || result.Longitude > tt.maxLon {
			t.Errorf("Locate(%q) returned implausible coordinate %.7f, %.7f", tt.input, result.Latitude, result.Longitude)
		}
		if !strings.Contains(result.Reference, "OpenStreetMap") || result.CoordinateSystem != "WGS-84" {
			t.Errorf("Locate(%q) missing source metadata: %#v", tt.input, result)
		}
	}
}

func TestLocateParsingAndCoverageErrors(t *testing.T) {
	for _, input := range []string{"", "G6 K1837+000", "S101 K42+000", "G7 K10+000", "G6 K1792+1000"} {
		if _, err := Locate(input); err == nil {
			t.Errorf("Locate(%q) expected an error", input)
		}
	}
	result, err := Locate("g6 k1792+200")
	if err != nil || result.Route != "G6" || result.Meter != 1792200 {
		t.Fatalf("case-insensitive input was not normalized: %#v, %v", result, err)
	}
}

func TestCoverageList(t *testing.T) {
	coverage := CoverageList()
	if len(coverage) != 2 {
		t.Fatalf("expected two coverage entries, got %d", len(coverage))
	}
	if coverage[0].ControlQty < 5 || coverage[1].ControlQty < 4 {
		t.Errorf("coverage control counts look incomplete: %#v", coverage)
	}
}

func TestLocateWithMapReturnsLocalActualRoads(t *testing.T) {
	data, err := LocateWithMap("G6 K1792+200")
	if err != nil {
		t.Fatalf("LocateWithMap returned error: %v", err)
	}
	if data.Result.Route != "G6" || data.RangeM != 180 {
		t.Fatalf("unexpected local map metadata: %#v", data)
	}
	if !data.MainlineMatched || data.MainlineOffsetM > 100 || data.HeadingDegrees < 0 || data.HeadingDegrees >= 360 {
		t.Fatalf("unexpected local map fit metadata: %#v", data)
	}
	if len(data.Roads) < 2 {
		t.Fatalf("expected local road geometries, got %d", len(data.Roads))
	}
	mainCount := 0
	nearestMainM := 1e9
	for _, road := range data.Roads {
		if len(road.Points) < 2 {
			t.Fatalf("road %d has incomplete geometry", road.OSMID)
		}
		if road.Kind == "main" {
			mainCount++
			points := make([]point, len(road.Points))
			for index, vertex := range road.Points {
				points[index] = point{Lat: vertex.Latitude, Lon: vertex.Longitude}
			}
			if distance := distanceToRoad(point{Lat: data.Result.Latitude, Lon: data.Result.Longitude}, points); distance < nearestMainM {
				nearestMainM = distance
			}
		}
	}
	if mainCount == 0 {
		t.Fatal("expected local G6 carriageway geometry")
	}
	t.Logf("G6 K1792+200 nearest public G6 carriageway: %.1f m", nearestMainM)
	if nearestMainM > 100 {
		t.Fatalf("resolved point is %.1f m from the nearest public G6 geometry", nearestMainM)
	}
}

func TestNetworkHealthCoversBothRoutes(t *testing.T) {
	health := NetworkHealthReport()
	if health.RoadWayQty < 1000 || health.RampWayQty < 300 || health.VertexQty < 8000 {
		t.Fatalf("embedded network unexpectedly sparse: %#v", health)
	}
	if len(health.RouteAudits) != 2 {
		t.Fatalf("expected G6 and S101 audits, got %#v", health.RouteAudits)
	}
	for _, audit := range health.RouteAudits {
		if audit.SampleQty < 100 || audit.ControlQty < 4 || audit.MainlineWayQty == 0 {
			t.Fatalf("incomplete route audit: %#v", audit)
		}
		t.Logf("%s: %d samples, mean %.1f m, max %.1f m", audit.Code, audit.SampleQty, audit.MeanMainlineOffsetM, audit.MaxMainlineOffsetM)
		if audit.MaxMainlineOffsetM > 100 {
			t.Fatalf("%s route fit drifted to %.1f m", audit.Code, audit.MaxMainlineOffsetM)
		}
	}
}
