package locator

import (
	"math"
	"sort"
)

type elevationSample struct {
	Meter     int
	Elevation int
}

// ElevationForSegment interpolates the offline terrain profile at a station.
// It returns false for routes without a sampled profile or stations outside it.
func ElevationForSegment(segmentID string, meter int) (int32, bool) {
	samples := elevationProfiles[segmentID]
	if len(samples) < 2 || meter < samples[0].Meter || meter > samples[len(samples)-1].Meter {
		return 0, false
	}
	index := sort.Search(len(samples), func(i int) bool { return samples[i].Meter >= meter })
	if samples[index].Meter == meter {
		return int32(samples[index].Elevation), true
	}
	before, after := samples[index-1], samples[index]
	fraction := float64(meter-before.Meter) / float64(after.Meter-before.Meter)
	return int32(math.Round(float64(before.Elevation) + fraction*float64(after.Elevation-before.Elevation))), true
}

// MeanElevationForSegment uses trapezoidal integration over station distance.
func MeanElevationForSegment(segmentID string) (float64, bool) {
	samples := elevationProfiles[segmentID]
	if len(samples) < 2 {
		return 0, false
	}
	var area float64
	for index := 1; index < len(samples); index++ {
		before, after := samples[index-1], samples[index]
		area += float64(before.Elevation+after.Elevation) * float64(after.Meter-before.Meter) / 2
	}
	return area / float64(samples[len(samples)-1].Meter-samples[0].Meter), true
}
