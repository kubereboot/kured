package timewindow

import (
	"strings"
	"testing"
)

func TestParseWeekdays(t *testing.T) {
	tests := []struct {
		input  string
		result string
	}{
		{"0,4", "Sun---------Thu------"},
		{"su,mo,tu", "SunMonTue------------"},
		{"sunday,tu,thu", "Sun---Tue---Thu------"},
		{"THURSDAY", "------------Thu------"},
		{"we,WED,WeDnEsDaY", "---------Wed---------"},
		{"", "---------------------"},
		{",,,", "---------------------"},
	}

	for _, tst := range tests {
		res, err := parseWeekdays(strings.Split(tst.input, ","))
		if err != nil {
			t.Errorf("Received error for input %s: %v", tst.input, err)
		} else if res.String() != tst.result {
			t.Errorf("Test %s: Expected %s got %s", tst.input, tst.result, res.String())
		}
	}
}

func TestParseWeekdaysErrors(t *testing.T) {
	tests := []string{
		"15",
		"-8",
		"8",
		"mon,tue,wed,fridayyyy",
	}

	for _, tst := range tests {
		_, err := parseWeekdays(strings.Split(tst, ","))
		if err == nil {
			t.Errorf("Expected to receive error for input %s", tst)
		}
	}
}
