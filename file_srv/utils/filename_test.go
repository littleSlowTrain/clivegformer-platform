package utils

import "testing"

func TestParseScientificFilename(t *testing.T) {
	tests := []struct {
		name       string
		classified bool
		region     string
		block      uint32
		year       uint32
	}{
		{"17001_0_20080101_20081230.nc", true, "17001", 0, 2008},
		{"00123_12_20120101_20121231.hdf", true, "00123", 12, 2012},
		{"17001_0_20080230_20081230.nc", false, "", 0, 0},
		{"17001_0_20080101_20091230.nc", false, "", 0, 0},
		{"17001_0_20081230_20080101.nc", false, "", 0, 0},
		{"17001_20080101_20081230.nc", false, "", 0, 0},
		{"analysis-result.nc", false, "", 0, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ParseScientificFilename(test.name)
			if got.Classified != test.classified || got.RegionCode != test.region || got.BlockIndex != test.block || got.DataYear != test.year {
				t.Fatalf("ParseScientificFilename(%q) = %+v", test.name, got)
			}
		})
	}
}
