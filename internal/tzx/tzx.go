package tzx

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/tz"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// GetCountryByName returns a single tz.Country that matches
// the country name provided and whether it was found.
func GetCountryByName(name string) (*tz.Country, bool) {
	if name == "" {
		return nil, false
	}

	name = cases.Title(language.English, cases.NoLower).String(name)
	name = strings.ReplaceAll(name, "Of", "of")
	name = strings.ReplaceAll(name, "And", "and")

	for _, i := range tz.GetCountries() {
		if i.Name == name {
			return &i, true
		}
	}

	return nil, false
}

// GetCountriesWithPrefix returns the list of countries
// that begin with the prefix s, for up to the first 5
// characters of the country name.
func GetCountriesWithPrefix(s string) []tz.Country {
	if s == "" {
		return nil
	}

	// Country name must be capitalized to match contents of tz.GetCountries()
	s = cases.Title(language.English, cases.NoLower).String(s)

	var (
		prefix string
		stop   string
	)

	// Handle up to the first 5 characters of the country name
	switch len(s) {
	case 1:
		prefix = s[0:1]
		stop = nextLetter(prefix)
	case 2:
		prefix = s[0:2]
		secondChar := s[1:2]
		next := nextLetter(secondChar)
		stop = fmt.Sprintf("%s%s", s[0:1], next)
	case 3:
		prefix = s[0:3]
		thirdChar := s[2:3]
		next := nextLetter(thirdChar)
		stop = strings.Join([]string{s[0:2], next}, "")
	case 4:
		prefix = s[0:4]
		fourthChar := s[3:4]
		next := nextLetter(fourthChar)
		stop = strings.Join([]string{s[0:3], next}, "")
	case 5:
		prefix = s[0:5]
		fifthChar := s[4:5]
		next := nextLetter(fifthChar)
		stop = strings.Join([]string{s[0:4], next}, "")
	}

	matches := make([]tz.Country, 0)

	countries := tz.GetCountries()
	for i := range countries {
		// Stop the loop once the stop prefix is reached
		if strings.HasPrefix(countries[i].Name, stop) {
			break
		}

		if strings.HasPrefix(countries[i].Name, prefix) {
			matches = append(matches, countries[i])
		}
	}

	return matches
}

// nextLetter returns the letter in the English alphabetic
// that comes after the provided letter l.
func nextLetter(l string) string {
	if len(l) != 1 {
		return ""
	}

	l = strings.ToLower(l)

	// Instead of returning '{', loop back around after Z
	if l == "z" {
		return "a"
	}

	r := []rune(l)

	return fmt.Sprintf("%c", r[0]+1)
}

// GetAbbreviatedTimezone returns the abbreviated
// timezone with UTC offset for the provided zone name
// in the following format: EST (UTC-05:00).
func GetAbbreviatedTimezone(name string) string {
	location, err := time.LoadLocation(name)
	if err != nil {
		return ""
	}

	now := time.Now().In(location)

	zone, _ := now.Zone()

	result := fmt.Sprintf("%s (UTC%s)", zone, now.Format("-07:00"))

	return result
}
