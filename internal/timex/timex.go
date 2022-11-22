package timex

import (
	"fmt"
	"strings"
	"time"
)

var (
	weekdays = map[string]time.Weekday{}
)

func init() {
	for d := time.Sunday; d <= time.Saturday; d++ {
		name := d.String()

		// support weekday long names
		weekdays[name] = d

		// support weekday short names (eg, Mon, Tues, etc.)
		weekdays[name[:3]] = d

		// support lowercase weekday names
		name = strings.ToLower(name)
		weekdays[name] = d
		weekdays[name[:3]] = d
	}
}

// ParseWeekday parses a weekday given by its name.
// Both long names such as "Monday", "Tuesday", etc.
// and short names such as "Mon", "Tue" are supported.
// Case is ignored.
func ParseWeekday(s string) (time.Weekday, error) {
	if d, ok := weekdays[s]; ok {
		return d, nil
	}

	return time.Sunday, fmt.Errorf("invalid weekday %q", s)
}

// NextWeekday calculates the timestamp of the next occurring weekday given a starting timestamp.
func NextWeekday(t time.Time, weekday string, hour int) time.Time {
	// Convert weekday to numeric representation
	d, _ := ParseWeekday(weekday)
	weekdayNumber := int(d)

	// Get weekday of current timestamp as numeric representation
	currentWeekdayNumber := int(t.Weekday())

	var diff int

	switch {
	case currentWeekdayNumber < weekdayNumber:
		diff = weekdayNumber - currentWeekdayNumber
	case currentWeekdayNumber > weekdayNumber:
		diff = 7 - (currentWeekdayNumber - weekdayNumber)
	case currentWeekdayNumber == weekdayNumber:
		diff = 7
	}

	timestamp := t.AddDate(0, 0, diff)

	// Set the hour on that date
	year, month, day := timestamp.Date()
	timestamp = time.Date(year, month, day, hour, 0, 0, 0, time.UTC)

	return timestamp
}
