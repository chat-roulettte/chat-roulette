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

		// support weekday short names (eg, Mon, Tue, etc.)
		weekdays[name[:3]] = d

		// support lowercase weekday names
		name = strings.ToLower(name)
		weekdays[name] = d
		weekdays[name[:3]] = d

		// Edge cases
		switch d {
		case time.Tuesday:
			weekdays["Tues"] = d
		case time.Thursday:
			weekdays["Thurs"] = d
		}
	}
}

// ParseWeekday parses a weekday given by its name.
// Both long names such as "Monday", "Tuesday", etc.
// and short names such as "Mon", "Tue" are supported.
// Case is ignored.
func ParseWeekday(s string) (time.Weekday, error) {
	day, ok := weekdays[s]
	if !ok {
		return time.Sunday, fmt.Errorf("invalid weekday %q", s)
	}

	return day, nil
}

// NextWeekday calculates the timestamp of the next occurring weekday from the given timestamp.
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

// NextMonth calculates the timestamp of the next occurrence in the following month from the given timestamp.
func NextMonth(t time.Time) time.Time {
	// Determine the weekday and ordinal occurrence
	weekday := t.Weekday()
	ordinal := (t.Day()-1)/7 + 1

	// Move to the next month
	month := t.Month() + 1
	year := t.Year()
	if month > 12 {
		month = 1
		year++
	}

	// Find the first day of the next month
	firstDayNextMonth := time.Date(year, month, 1, t.Hour(), t.Minute(), 0, 0, t.Location())

	// Find the first occurrence of the specified weekday in the next month
	firstWeekday := firstDayNextMonth
	for firstWeekday.Weekday() != t.Weekday() {
		firstWeekday = firstWeekday.AddDate(0, 0, 1)
	}

	// Calculate the maximum number of occurrences of the specified weekday in the next month
	maxOccurrences := 0
	for d := firstDayNextMonth; d.Month() == month; d = d.AddDate(0, 0, 1) {
		if d.Weekday() == weekday {
			maxOccurrences++
		}
	}

	// Handle edge case: current month has 4 weeks, but next month has 5 weeks
	if ordinal == 4 && maxOccurrences == 5 {
		ordinal = 5
	}

	// Calculate the date in the next month based on the ordinal
	timestamp := firstWeekday.AddDate(0, 0, (ordinal-1)*7)

	return timestamp
}

// FormatMonthlyOccurrence extracts the day and week number from a timestamp
// and formats it into a human-readable string for when in the month that it occurs.
//
// eg, "first Monday" or "last Saturday"
func FormatMonthlyOccurrence(t time.Time) string {
	week := (t.Day()-1)/7 + 1

	var occurrence string
	switch week {
	case 1:
		occurrence = "first"
	case 2:
		occurrence = "second"
	case 3:
		occurrence = "third"
	default:
		occurrence = "last"
	}

	formattedString := fmt.Sprintf("%s %s", occurrence, t.Weekday())

	return formattedString
}
