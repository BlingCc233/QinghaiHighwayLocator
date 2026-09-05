package locator

import "strings"

// RouteSegment is the operator-facing jurisdiction catalog. Coordinates for
// supplied control points are retained as anchors; the remaining path is
// derived from the embedded public road snapshot and snapped to matching ways.
type RouteSegment struct {
	ID          string `json:"id"`
	Brigade     string `json:"brigade"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	SegmentName string `json:"segmentName"`
	Start       string `json:"start"`
	End         string `json:"end"`
	StartMeter  int    `json:"startMeter"`
	EndMeter    int    `json:"endMeter"`
	ControlQty  int    `json:"controlQty"`
}

type routeAnchor struct {
	meter int
	name  string
	point point
}

type segmentDefinition struct {
	RouteSegment
	anchors []routeAnchor
}

var segmentDefinitions = []segmentDefinition{
	{RouteSegment: RouteSegment{ID: "xjk-g6-pingxi", Brigade: "韵家口大队", Code: "G6", Name: "京藏高速公路", SegmentName: "平安—西宁", Start: "K1766+600", End: "K1800+500", StartMeter: 1766600, EndMeter: 1800500}, anchors: []routeAnchor{{1766600, "平安收费站", point{36.51543397, 102.10815501}}, {1800500, "朝阳互通立交", point{36.6398609, 101.7821288}}}},
	{RouteSegment: RouteSegment{ID: "xjk-g6-xiguojing", Brigade: "韵家口大队", Code: "G6", Name: "京藏高速公路西过境段", SegmentName: "西过境段", Start: "K1800+500", End: "K1836+000", StartMeter: 1800500, EndMeter: 1836000}, anchors: []routeAnchor{{1800500, "朝阳互通立交", point{36.6398609, 101.7821288}}, {1836000, "西宁西收费站", point{36.6591093, 101.4404679}}}},
	{RouteSegment: RouteSegment{ID: "xjk-s101", Brigade: "韵家口大队", Code: "S101", Name: "西宁高速", SegmentName: "韵家口—互助", Start: "K0+000", End: "K41+430", StartMeter: 0, EndMeter: 41430}, anchors: []routeAnchor{{0, "G6衔接处", point{36.5822891, 101.8569051}}, {15000, "塘川收费站", point{36.70137166, 101.89920024}}, {41430, "互助端", point{36.8873869, 102.0416487}}}},
	{RouteSegment: RouteSegment{ID: "dt-g0611-xining-datong", Brigade: "大通大队", Code: "G0611", Name: "张汶高速", SegmentName: "西宁—大通", Start: "K273+000", End: "K301+000", StartMeter: 273000, EndMeter: 301000}, anchors: []routeAnchor{{273000, "西宁北方向", point{36.735, 101.780}}, {283500, "西宁北收费站", point{36.830, 101.770}}, {301000, "大通端", point{37.020, 101.760}}}},
	{RouteSegment: RouteSegment{ID: "dt-g0611-datong-mainline", Brigade: "大通大队", Code: "G0611", Name: "张汶高速", SegmentName: "大通主线收费站段", Start: "K30+000", End: "K34+600", StartMeter: 30000, EndMeter: 34600}, anchors: []routeAnchor{{30000, "大通南端", point{36.930, 101.680}}, {34000, "大通主线收费站", point{36.980, 101.700}}, {34600, "大通端", point{37.000, 101.710}}}},
	{RouteSegment: RouteSegment{ID: "dt-g569", Brigade: "大通大队", Code: "G569", Name: "曼德拉至大通公路", SegmentName: "大通—东峡段", Start: "K252+800", End: "K273+000", StartMeter: 252800, EndMeter: 273000}, anchors: []routeAnchor{{252800, "东峡收费站", point{37.06039050, 101.82686645}}, {273000, "东峡方向", point{37.180, 101.930}}}},
	{RouteSegment: RouteSegment{ID: "nr-g0612-main", Brigade: "南绕城大队", Code: "G0612", Name: "西和高速", SegmentName: "曹家堡—西宁西", Start: "K0+000", End: "K59+760", StartMeter: 0, EndMeter: 59760}, anchors: []routeAnchor{{0, "曹家堡端", point{36.4951901, 102.0770733}}, {59760, "扎麻隆互通", point{36.6598644, 101.4381784}}}},
	{RouteSegment: RouteSegment{ID: "nr-g0612-east", Brigade: "南绕城大队", Code: "G0612", Name: "西和高速东延段", SegmentName: "东延段", Start: "K0+000", End: "K9+000", StartMeter: 0, EndMeter: 9000}, anchors: []routeAnchor{{0, "海东端", point{36.4951901, 102.0770733}}, {9000, "东延端", point{36.5464458, 101.9380338}}}},
	{RouteSegment: RouteSegment{ID: "hz-s104-xining", Brigade: "湟中大队", Code: "S104", Name: "西贵公路", SegmentName: "西宁—湟中", Start: "K0+000", End: "K21+900", StartMeter: 0, EndMeter: 21900}, anchors: []routeAnchor{{0, "西宁端", point{36.582, 101.856}}, {16800, "西宁南收费站", point{36.550, 101.650}}, {21900, "湟中端", point{36.520, 101.620}}}},
	{RouteSegment: RouteSegment{ID: "hz-s104-guid", Brigade: "湟中大队", Code: "S104", Name: "西贵公路", SegmentName: "湟中—贵德", Start: "K21+900", End: "K85+600", StartMeter: 21900, EndMeter: 85600}, anchors: []routeAnchor{{21900, "湟中端", point{36.520, 101.620}}, {85600, "黄河清大桥", point{36.059323, 101.453900}}}},
	{RouteSegment: RouteSegment{ID: "hy-g6-xining-huangyuan", Brigade: "湟源大队", Code: "G6", Name: "京藏高速公路", SegmentName: "西宁—湟源", Start: "K1836+000", End: "K1851+300", StartMeter: 1836000, EndMeter: 1851300}, anchors: []routeAnchor{{1836000, "西宁西收费站", point{36.6591093, 101.4404679}}, {1851300, "湟源收费站", point{36.69011518, 101.29110186}}}},
	{RouteSegment: RouteSegment{ID: "hy-g6-huangyuan-daotanghe", Brigade: "湟源大队", Code: "G6", Name: "京藏高速公路", SegmentName: "湟源—倒淌河", Start: "K1851+300", End: "K1900+100", StartMeter: 1851300, EndMeter: 1900100}, anchors: []routeAnchor{{1851300, "湟源收费站", point{36.69011518, 101.29110186}}, {1900100, "倒淌河匝道收费站", point{36.401, 100.990}}}},
	{RouteSegment: RouteSegment{ID: "dtcg-g341", Brigade: "大通城关大队", Code: "G341", Name: "胶海线", SegmentName: "阿家堡—西海镇", Start: "K2312+800", End: "K2397+000", StartMeter: 2312800, EndMeter: 2397000}, anchors: []routeAnchor{{2312800, "阿家堡端", point{36.8403924, 101.9709645}}, {2319889, "塔尔收费站", point{36.8822790, 102.0284462}}, {2397000, "西海镇方向", point{36.8802128, 102.5666808}}}},
}

func RouteCatalog() []RouteSegment {
	items := make([]RouteSegment, len(segmentDefinitions))
	for i, definition := range segmentDefinitions {
		items[i] = definition.RouteSegment
		items[i].ControlQty = len(definition.anchors)
	}
	return items
}

func routeDefinition(id string) (segmentDefinition, bool) {
	id = strings.TrimSpace(id)
	for _, definition := range segmentDefinitions {
		if definition.ID == id {
			return definition, true
		}
	}
	return segmentDefinition{}, false
}
