package timewindow

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var dayStrings = map[string]time.Weekday{
	"su":        time.Sunday,
	"sun":       time.Sunday,
	"sunday":    time.Sunday,
	"mo":        time.Monday,
	"mon":       time.Monday,
	"monday":    time.Monday,
	"tu":        time.Tuesday,
	"tue":       time.Tuesday,
	"tuesday":   time.Tuesday,
	"we":        time.Wednesday,
	"wed":       time.Wednesday,
	"wednesday": time.Wednesday,
	"th":        time.Thursday,
	"thu":       time.Thursday,
	"thursday":  time.Thursday,
	"fr":        time.Friday,
	"fri":       time.Friday,
	"friday":    time.Friday,
	"sa":        time.Saturday,
	"sat":       time.Saturday,
	"saturday":  time.Saturday,
}

type weekdays []time.Weekday

func parseWeekdays(days []string) (weekdays, error) {
	var result []time.Weekday
	for _, day := range days {
		weekday, err := parseWeekday(day)
		if err != nil {
			return nil, err
		}

		result = append(result, weekday)
	}

	return weekdays(result), nil
}

func (w weekdays) Contains(day time.Weekday) bool {
	for _, d := range w {
		if d == day {
			return true
		}
	}

	return false
}

func (w weekdays) String() string {
	var days []string
	for _, d := range w {
		days = append(days, d.String())
	}

	return strings.Join(days, ",")
}

func parseWeekday(day string) (time.Weekday, error) {
	if n, err := strconv.Atoi(day); err == nil {
		if n >= 0 && n < 7 {
			return time.Weekday(n), nil
		} else {
			return time.Sunday, fmt.Errorf("Invalid weekday, number out of range: %s", day)
		}
	}

	if day, ok := dayStrings[strings.ToLower(day)]; ok {
		return day, nil
	} else {
		return time.Sunday, fmt.Errorf("Invalid weekday: %s", day)
	}
}
