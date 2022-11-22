package templatex

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

// Capitalize returns a copy of the string with the first letter capitalized.
//
// strings.Title is not used because it is deprecated.
func Capitalize(source string) string {
	return cases.Title(language.English, cases.NoLower).String(source)
}

func CapitalizeInterval(i models.IntervalEnum) string {
	return Capitalize(i.String())
}

// PrettyDate returns a date in the following format:
// January 4th, 2022
func PrettyDate(t time.Time) string {
	year := t.Year()
	day := humanize.Ordinal(t.Day())
	month := t.Format("January")

	return fmt.Sprintf("%s %s, %d", month, day, year)
}

// PrettierDate returns a date in the following format:
// Monday, January 4th, 2021
func PrettierDate(t time.Time) string {
	year := t.Year()
	day := humanize.Ordinal(t.Day())
	first := t.Format("Monday, January")

	return fmt.Sprintf("%s %s, %d", first, day, year)
}

// DerefBool derefences a pointer to a boolean.
func DerefBool(b *bool) bool {
	return *b
}
