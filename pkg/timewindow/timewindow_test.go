package timewindow

import (
	"strings"
	"testing"
	"time"
)

func TestTimeWindows(t *testing.T) {
	type testcase struct {
		time   string
		result bool
	}

	tests := []struct {
		days  string
		start string
		end   string
		loc   string
		cases []testcase
	}{
		{"mon,tue,wed,thu,fri", "9am", "5pm", "America/Los_Angeles", []testcase{
			{"2019/03/31 10:00 PDT", false},
			{"2019/04/04 00:49 PDT", false},
			{"2019/04/04 12:00 PDT", true},
			{"2019/04/04 11:59 UTC", false},
			{"2019/04/05 08:59 PDT", false},
			{"2019/04/05 9:01 PDT", true},
		}},
		{"mon,we,fri", "10:01", "11:30am", "America/Los_Angeles", []testcase{
			{"2019/04/05 10:30 PDT", true},
			{"2019/04/06 10:30 PDT", false},
			{"2019/04/07 10:30 PDT", false},
			{"2019/04/08 10:30 PDT", true},
			{"2019/04/09 10:30 PDT", false},
			{"2019/04/10 10:30 PDT", true},
			{"2019/04/11 10:30 PDT", false},
		}},
		{"mo,tu,we,th,fr", "00:00", "23:59:59", "UTC", []testcase{
			{"2019/04/18 00:00 UTC", true},
			{"2019/04/18 23:59 UTC", true},
		}},
		{"mon,tue,wed,thu,fri", "9pm", "5am", "America/Los_Angeles", []testcase{
			{"2019/03/30 04:00 PDT", false},
			{"2019/03/31 10:00 PDT", false},
			{"2019/03/31 22:00 PDT", false},
			{"2019/04/04 00:49 PDT", true},
			{"2019/04/04 12:00 PDT", false},
			{"2019/04/04 22:49 PDT", true},
			{"2019/04/05 00:49 PDT", true},
			{"2019/04/05 08:59 PDT", false},
			{"2019/04/05 9:01 PDT", false},
		}},
		{"mon,tue,wed,thu,fri", "11:59pm", "00:01am", "America/Los_Angeles", []testcase{
			{"2019/04/04 23:58 PDT", false},
			{"2019/04/04 23:59 PDT", true},
			{"2019/04/05 00:00 PDT", true},
			{"2019/04/05 00:01 PDT", true},
			{"2019/04/05 00:02 PDT", false},
		}},
		{"mon,tue,wed,fri", "11:59pm", "00:01am", "America/Los_Angeles", []testcase{
			{"2019/04/04 23:58 PDT", false},
			{"2019/04/04 23:59 PDT", false}, // Even that this falls in the between the hours Thursday is not included so should not run
			{"2019/04/05 00:00 PDT", true},
			{"2019/04/05 00:02 PDT", false},
		}},
		{"mon,tue,wed,thu", "11:59pm", "00:01am", "America/Los_Angeles", []testcase{
			{"2019/04/04 23:58 PDT", false},
			{"2019/04/04 23:59 PDT", true},
			{"2019/04/05 00:00 PDT", false}, // Even that this falls in the between the hours Friday is not included so should not run
			{"2019/04/05 00:02 PDT", false},
		}},
		{"mon,tue,wed,thu,fri", "11:59pm", "00:01am", "UTC", []testcase{
			{"2019/04/04 23:58 UTC", false},
			{"2019/04/04 23:59 UTC", true},
			{"2019/04/05 00:00 UTC", true},
			{"2019/04/05 00:01 UTC", true},
			{"2019/04/05 00:02 UTC", false},
		}},
	}

	for i, tst := range tests {
		tw, err := New(strings.Split(tst.days, ","), tst.start, tst.end, tst.loc)
		if err != nil {
			t.Errorf("Test [%d] failed to create TimeWindow: %v", i, err)
		}

		for _, cas := range tst.cases {
			tm, err := time.ParseInLocation("2006/01/02 15:04 MST", cas.time, tw.location)
			if err != nil {
				t.Errorf("Failed to parse time \"%s\": %v", cas.time, err)
			} else if cas.result != tw.Contains(tm) {
				t.Errorf("(%s) contains (%s) didn't match expected result of %v", tw.String(), cas.time, cas.result)
			}
		}
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input  string
		hour   int
		minute int
	}{
		// Formats listed in the kured documentation.
		{"5pm", 17, 0},
		{"5:00pm", 17, 0},
		{"17:00", 17, 0},
		{"17", 17, 0},
		// Other 12-hour clock times, with and without a leading zero on the hour.
		{"9:30am", 9, 30},
		{"09:30am", 9, 30},
		{"5:45pm", 17, 45},
		{"05:45pm", 17, 45},
		{"11:59pm", 23, 59},
		{"12:00am", 0, 0},
		{"12:30pm", 12, 30},
	}

	for _, tst := range tests {
		got, err := parseTime(tst.input, time.UTC)
		if err != nil {
			t.Errorf("parseTime(%q) returned an error: %v", tst.input, err)
			continue
		}
		if got.Hour() != tst.hour || got.Minute() != tst.minute {
			t.Errorf("parseTime(%q) = %02d:%02d, expected %02d:%02d", tst.input, got.Hour(), got.Minute(), tst.hour, tst.minute)
		}
	}
}

func TestParseTimeErrors(t *testing.T) {
	tests := []string{
		"",
		"noon",
		"13pm",
		"5:60pm",
		"25:00",
	}

	for _, tst := range tests {
		if _, err := parseTime(tst, time.UTC); err == nil {
			t.Errorf("Expected an error for input %q", tst)
		}
	}
}

func TestTimeWindowWithTwelveHourMinutes(t *testing.T) {
	tw, err := New([]string{"mon", "tue", "wed", "thu", "fri"}, "9:30am", "5:15pm", "UTC")
	if err != nil {
		t.Fatalf("Failed to create TimeWindow: %v", err)
	}

	tests := []struct {
		time   string
		result bool
	}{
		{"2019/04/04 09:29 UTC", false},
		{"2019/04/04 09:30 UTC", true},
		{"2019/04/04 17:15 UTC", true},
		{"2019/04/04 17:16 UTC", false},
	}

	for _, tst := range tests {
		tm, err := time.ParseInLocation("2006/01/02 15:04 MST", tst.time, time.UTC)
		if err != nil {
			t.Fatalf("Failed to parse time %q: %v", tst.time, err)
		}
		if got := tw.Contains(tm); got != tst.result {
			t.Errorf("(%s) contains (%s) = %v, expected %v", tw.String(), tst.time, got, tst.result)
		}
	}
}
