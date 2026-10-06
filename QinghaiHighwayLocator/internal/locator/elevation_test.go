package locator

import (
	"math"
	"testing"
)

func TestYunjiaKouElevationProfiles(t *testing.T) {
	wants := map[string]float64{
		"xjk-g6-pingxi":    2172.4,
		"xjk-g6-xiguojing": 2351.0,
		"xjk-s101":         2422.3,
	}
	var area, length float64
	for _, segment := range RouteCatalog() {
		if segment.Brigade != "韵家口大队" {
			continue
		}
		samples := elevationProfiles[segment.ID]
		if len(samples) < 2 || samples[0].Meter != segment.StartMeter || samples[len(samples)-1].Meter != segment.EndMeter {
			t.Fatalf("%s profile does not cover the full segment", segment.ID)
		}
		for i := 1; i < len(samples); i++ {
			if gap := samples[i].Meter - samples[i-1].Meter; gap <= 0 || gap > 100 {
				t.Fatalf("%s invalid sample gap %d", segment.ID, gap)
			}
		}
		mean, ok := MeanElevationForSegment(segment.ID)
		if !ok || math.Abs(mean-wants[segment.ID]) > 0.1 {
			t.Errorf("%s mean %.2f, want %.1f", segment.ID, mean, wants[segment.ID])
		}
		lengthM := float64(segment.EndMeter - segment.StartMeter)
		area += mean * lengthM
		length += lengthM
		result, err := LocateForSegment(segment.ID, segment.Start)
		if err != nil || result.ElevationMeters == nil || *result.ElevationMeters != int32(samples[0].Elevation) {
			t.Errorf("%s start elevation is not propagated to Result: %+v, %v", segment.ID, result.ElevationMeters, err)
		}
	}
	if math.Abs(area/length-2323.0) > 0.1 {
		t.Errorf("brigade mean %.2f, want 2323.0", area/length)
	}
}

func TestElevationInterpolationAndBounds(t *testing.T) {
	samples := elevationProfiles["xjk-g6-pingxi"]
	first, second := samples[0], samples[1]
	actual, ok := ElevationForSegment("xjk-g6-pingxi", first.Meter+(second.Meter-first.Meter)/2)
	want := int32(math.Round(float64(first.Elevation+second.Elevation) / 2))
	if !ok || actual != want {
		t.Errorf("interpolated altitude = %d, %t; want %d", actual, ok, want)
	}
	for _, meter := range []int{first.Meter - 1, samples[len(samples)-1].Meter + 1} {
		if _, ok := ElevationForSegment("xjk-g6-pingxi", meter); ok {
			t.Errorf("elevation unexpectedly available at %d", meter)
		}
	}
	if _, ok := ElevationForSegment("dt-g0611-xining-datong", 273000); ok {
		t.Fatal("unsampled segment unexpectedly has elevation")
	}
}
