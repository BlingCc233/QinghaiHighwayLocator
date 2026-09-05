package locator

import (
	"container/heap"
	"math"
	"sort"
	"sync"
)

// networkRouteGraph stitches the independently downloaded OSM ways at their
// shared vertices. It keeps the route geometry on the public road centerline
// instead of drawing a straight line between control points.
type networkRouteGraph struct {
	points []point
	edges  [][]networkRouteEdge
}

type networkRouteEdge struct {
	to     int
	weight float64
}

type networkRouteNodeKey struct {
	lat int64
	lon int64
}

type networkRouteQueueItem struct {
	node     int
	distance float64
}

type networkRouteQueue []networkRouteQueueItem

func (q networkRouteQueue) Len() int { return len(q) }
func (q networkRouteQueue) Less(i, j int) bool {
	return q[i].distance < q[j].distance
}
func (q networkRouteQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *networkRouteQueue) Push(item any) {
	*q = append(*q, item.(networkRouteQueueItem))
}
func (q *networkRouteQueue) Pop() any {
	old := *q
	item := old[len(old)-1]
	*q = old[:len(old)-1]
	return item
}

type networkRouteCacheEntry struct {
	route routeData
	ok    bool
}

var (
	networkGraphCache sync.Map
	networkRouteCache sync.Map
)

func networkRouteForDefinition(definition segmentDefinition) (routeData, bool) {
	if cached, ok := networkRouteCache.Load(definition.ID); ok {
		entry := cached.(networkRouteCacheEntry)
		return entry.route, entry.ok
	}

	route, ok := buildNetworkRoute(definition)
	networkRouteCache.Store(definition.ID, networkRouteCacheEntry{route: route, ok: ok})
	return route, ok
}

func networkGraphForCode(code string) networkRouteGraph {
	if cached, ok := networkGraphCache.Load(code); ok {
		return cached.(networkRouteGraph)
	}
	graph := buildNetworkRouteGraph(code)
	networkGraphCache.Store(code, graph)
	return graph
}

func buildNetworkRouteGraph(code string) networkRouteGraph {
	graph := networkRouteGraph{}
	index := make(map[networkRouteNodeKey]int)
	addPoint := func(p point) int {
		key := networkRouteNodeKey{
			lat: int64(math.Round(p.Lat * 1e7)),
			lon: int64(math.Round(p.Lon * 1e7)),
		}
		if node, ok := index[key]; ok {
			return node
		}
		node := len(graph.points)
		index[key] = node
		graph.points = append(graph.points, p)
		graph.edges = append(graph.edges, nil)
		return node
	}

	for _, road := range routeMainlines(code) {
		for i := 1; i < len(road.Points); i++ {
			from := addPoint(road.Points[i-1])
			to := addPoint(road.Points[i])
			if from == to {
				continue
			}
			weight := distance(graph.points[from], graph.points[to])
			graph.edges[from] = append(graph.edges[from], networkRouteEdge{to: to, weight: weight})
			graph.edges[to] = append(graph.edges[to], networkRouteEdge{to: from, weight: weight})
		}
	}
	stitchNearbyComponents(&graph, code)
	return graph
}

// stitchNearbyComponents repairs small gaps introduced when OSM stores
// opposing carriageways or independently edited way sequences as separate
// graph components. Only close components are joined.
func stitchNearbyComponents(graph *networkRouteGraph, code string) {
	if len(graph.points) == 0 {
		return
	}
	maxDistance := 60.0
	if code == "G341" {
		maxDistance = 80.0
	}
	components := networkGraphComponents(*graph)
	componentOf := make([]int, len(graph.points))
	for component, nodes := range components {
		for _, node := range nodes {
			componentOf[node] = component
		}
	}

	const cellSize = 0.0005
	type cellKey struct{ lat, lon int64 }
	cells := make(map[cellKey][]int, len(graph.points))
	for node, p := range graph.points {
		key := cellKey{lat: int64(math.Floor(p.Lat / cellSize)), lon: int64(math.Floor(p.Lon / cellSize))}
		cells[key] = append(cells[key], node)
	}
	for node, p := range graph.points {
		base := cellKey{lat: int64(math.Floor(p.Lat / cellSize)), lon: int64(math.Floor(p.Lon / cellSize))}
		for dLat := int64(-1); dLat <= 1; dLat++ {
			for dLon := int64(-1); dLon <= 1; dLon++ {
				for _, other := range cells[cellKey{lat: base.lat + dLat, lon: base.lon + dLon}] {
					if other <= node || componentOf[other] == componentOf[node] {
						continue
					}
					weight := distance(p, graph.points[other])
					if weight > maxDistance || networkGraphHasEdge(graph.edges[node], other) {
						continue
					}
					graph.edges[node] = append(graph.edges[node], networkRouteEdge{to: other, weight: weight})
					graph.edges[other] = append(graph.edges[other], networkRouteEdge{to: node, weight: weight})
				}
			}
		}
	}
}

func networkGraphComponents(graph networkRouteGraph) [][]int {
	seen := make([]bool, len(graph.points))
	components := make([][]int, 0, len(graph.points)/8)
	for start := range graph.points {
		if seen[start] {
			continue
		}
		component := make([]int, 0, 64)
		queue := []int{start}
		seen[start] = true
		for len(queue) > 0 {
			node := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			component = append(component, node)
			for _, edge := range graph.edges[node] {
				if !seen[edge.to] {
					seen[edge.to] = true
					queue = append(queue, edge.to)
				}
			}
		}
		components = append(components, component)
	}
	return components
}

func networkGraphHasEdge(edges []networkRouteEdge, target int) bool {
	for _, edge := range edges {
		if edge.to == target {
			return true
		}
	}
	return false
}

func buildNetworkRoute(definition segmentDefinition) (routeData, bool) {
	if len(definition.anchors) < 2 {
		return routeData{}, false
	}
	graph := networkGraphForCode(definition.Code)
	if len(graph.points) < 2 {
		return routeData{}, false
	}
	startCandidates := nearestNetworkNodes(graph, definition.anchors[0].point, 32)
	endCandidates := nearestNetworkNodes(graph, definition.anchors[len(definition.anchors)-1].point, 32)
	if len(startCandidates) == 0 || len(endCandidates) == 0 {
		return routeData{}, false
	}

	minimumSpan := math.Max(1000, float64(definition.EndMeter-definition.StartMeter)*0.35)
	bestDistance := math.Inf(1)
	var bestPoints []point
	var bestControls []controlPoint
	for _, start := range startCandidates {
		distances, previous := shortestNetworkPaths(graph, start)
		for _, end := range endCandidates {
			if math.IsInf(distances[end], 1) || distances[end] >= bestDistance {
				continue
			}
			points := reconstructNetworkPath(graph, previous, start, end)
			if len(points) < 2 {
				continue
			}
			controls, ok := networkRouteControls(points, definition.anchors)
			if !ok || controls[len(controls)-1].RawMeter < minimumSpan {
				continue
			}
			bestDistance = distances[end]
			bestPoints = points
			bestControls = controls
		}
	}
	if len(bestPoints) < 2 || len(bestControls) < 2 {
		return routeData{}, false
	}

	return routeData{
		ID:          definition.ID,
		Brigade:     definition.Brigade,
		Code:        definition.Code,
		Name:        definition.Name,
		StartMeter:  definition.StartMeter,
		EndMeter:    definition.EndMeter,
		Points:      bestPoints,
		Controls:    bestControls,
		SnapNetwork: false,
	}, true
}

func nearestNetworkNodes(graph networkRouteGraph, target point, limit int) []int {
	type candidate struct {
		node int
		dist float64
	}
	candidates := make([]candidate, len(graph.points))
	for i, candidatePoint := range graph.points {
		candidates[i] = candidate{node: i, dist: distance(target, candidatePoint)}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].dist < candidates[j].dist })
	if limit > len(candidates) {
		limit = len(candidates)
	}
	result := make([]int, 0, limit)
	for _, candidate := range candidates[:limit] {
		result = append(result, candidate.node)
	}
	return result
}

func shortestNetworkPaths(graph networkRouteGraph, start int) ([]float64, []int) {
	distances := make([]float64, len(graph.points))
	previous := make([]int, len(graph.points))
	for i := range distances {
		distances[i] = math.Inf(1)
		previous[i] = -1
	}
	distances[start] = 0
	queue := &networkRouteQueue{{node: start, distance: 0}}
	heap.Init(queue)
	for queue.Len() > 0 {
		item := heap.Pop(queue).(networkRouteQueueItem)
		if item.distance > distances[item.node] {
			continue
		}
		for _, edge := range graph.edges[item.node] {
			candidate := item.distance + edge.weight
			if candidate >= distances[edge.to] {
				continue
			}
			distances[edge.to] = candidate
			previous[edge.to] = item.node
			heap.Push(queue, networkRouteQueueItem{node: edge.to, distance: candidate})
		}
	}
	return distances, previous
}

func reconstructNetworkPath(graph networkRouteGraph, previous []int, start, end int) []point {
	if start == end {
		return nil
	}
	nodes := make([]int, 0, 128)
	for node := end; node >= 0; node = previous[node] {
		nodes = append(nodes, node)
		if node == start {
			break
		}
	}
	if len(nodes) == 0 || nodes[len(nodes)-1] != start {
		return nil
	}
	points := make([]point, len(nodes))
	for i, node := range nodes {
		points[len(nodes)-1-i] = graph.points[node]
	}
	return points
}

func networkRouteControls(points []point, anchors []routeAnchor) ([]controlPoint, bool) {
	if len(points) < 2 || len(anchors) < 2 {
		return nil, false
	}
	rawMeters := make([]float64, len(anchors))
	for i, anchor := range anchors {
		rawMeters[i], _ = routeProjection(points, anchor.point)
		if i > 0 && rawMeters[i] <= rawMeters[i-1]+1 {
			return nil, false
		}
	}
	base := rawMeters[0]
	controls := make([]controlPoint, len(anchors))
	for i, anchor := range anchors {
		controls[i] = controlPoint{Meter: anchor.meter, Name: anchor.name, RawMeter: rawMeters[i] - base}
	}
	return controls, true
}

func routeProjection(points []point, target point) (float64, float64) {
	bestDistance := math.Inf(1)
	bestRaw := 0.0
	walked := 0.0
	for i := 1; i < len(points); i++ {
		segmentLength := distance(points[i-1], points[i])
		if segmentLength == 0 {
			continue
		}
		candidate, candidateDistance := nearestPointOnSegment(target, points[i-1], points[i])
		fraction := routeSegmentFraction(candidate, points[i-1], points[i])
		if candidateDistance < bestDistance {
			bestDistance = candidateDistance
			bestRaw = walked + segmentLength*fraction
		}
		walked += segmentLength
	}
	return bestRaw, bestDistance
}

func routeSegmentFraction(candidate, a, b point) float64 {
	segmentLength := distance(a, b)
	if segmentLength == 0 {
		return 0
	}
	return math.Max(0, math.Min(1, distance(a, candidate)/segmentLength))
}
