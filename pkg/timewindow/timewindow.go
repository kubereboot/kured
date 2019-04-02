package timewindow

import (
	"fmt"
	"time"
)

// Represents a time window
type TimeWindow struct {
	days      weekdays
	location  *time.Location
	startTime time.Time
	endTime   time.Time
}

func New(days []string, startTime, endTime, location string) (*TimeWindow, error) {
	tw := &TimeWindow{}

	var err error
	if tw.days, err = parseWeekdays(days); err != nil {
		return nil, err
	}

	if tw.location, err = time.LoadLocation(location); err != nil {
		return nil, err
	}

	if tw.startTime, err = parseTime(startTime, tw.location); err != nil {
		return nil, err
	}

	if tw.endTime, err = parseTime(endTime, tw.location); err != nil {
		return nil, err
	}

	return tw, nil
}

func (tw *TimeWindow) Contains(t time.Time) bool {
	loctime := t.In(tw.location)
	if !tw.days.Contains(loctime.Weekday()) {
		return false
	}

	start := time.Date(loctime.Year(), loctime.Month(), loctime.Day(), tw.startTime.Hour(), tw.startTime.Minute(), tw.startTime.Second(), 0, tw.location)
	end := time.Date(loctime.Year(), loctime.Month(), loctime.Day(), tw.endTime.Hour(), tw.endTime.Minute(), tw.endTime.Second(), 0, tw.location)

	return loctime.After(start) && loctime.Before(end)
}

func (tw *TimeWindow) String() string {
	return fmt.Sprintf("%s between %02d:%02d and %02d:%02d %s", tw.days.String(), tw.startTime.Hour(), tw.startTime.Minute(), tw.endTime.Hour(), tw.endTime.Minute(), tw.location.String())
}

func parseTime(s string, loc *time.Location) (time.Time, error) {
	fmts := []string{"15:04", "15:04:06", "03:04pm", "15", "03pm"}
	for _, f := range fmts {
		if t, err := time.ParseInLocation(f, s, loc); err == nil {
			return t, nil
		}
	}

	return time.Now(), fmt.Errorf("Invalid time format: %s", s)
}