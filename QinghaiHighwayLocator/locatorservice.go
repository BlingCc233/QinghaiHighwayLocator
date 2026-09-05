package main

import (
	"stationnum2omap/internal/locator"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type LocatorService struct{}

func (s *LocatorService) Locate(station string) (locator.Result, error) {
	return locator.Locate(station)
}

func (s *LocatorService) GetCoverage() []locator.Coverage {
	return locator.CoverageList()
}

func (s *LocatorService) GetRouteCatalog() []locator.RouteSegment {
	return locator.RouteCatalog()
}

func (s *LocatorService) LocateWithMap(station string) (locator.LocalMap, error) {
	return locator.LocateWithMap(station)
}

func (s *LocatorService) LocateWithMapForSegment(segmentID, station string) (locator.LocalMap, error) {
	return locator.LocateWithMapForSegment(segmentID, station)
}

func (s *LocatorService) GetNetworkHealth() locator.NetworkHealth {
	return locator.NetworkHealthReport()
}

func (s *LocatorService) GetOmapExportDirectory() string {
	return locator.DefaultOmapDataDirectory()
}

func (s *LocatorService) PickOmapDataDirectory() (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle("选择奥维 data 文件夹").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		PromptForSingleSelection()
}

func (s *LocatorService) PickAttachments() ([]string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle("选择路产附件").
		CanChooseFiles(true).
		CanChooseDirectories(false).
		PromptForMultipleSelection()
}

func (s *LocatorService) ExportOmap(input locator.OmapPointInput) (locator.OmapExportResult, error) {
	return locator.ExportOmap(input)
}

func (s *LocatorService) ExportOmapRange(input locator.OmapRangeInput) (locator.OmapRangeResult, error) {
	return locator.ExportOmapRange(input)
}
