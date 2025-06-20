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
// that begin with the prefix s, for up to the first 7
// characters of the country name.
func GetCountriesWithPrefix(s string) []tz.Country {
	if s == "" {
		return nil
	}

	s = cases.Title(language.English, cases.NoLower).String(s)
	maxLen := 7
	prefix := s[:min(len(s), maxLen)]
	stop := generateStopPrefix(prefix)

	countries := tz.GetCountries()
	matches := make([]tz.Country, 0, len(countries))

	for _, country := range countries {
		if strings.HasPrefix(country.Name, stop) {
			break
		}
		if strings.HasPrefix(country.Name, prefix) {
			matches = append(matches, country)
		}
	}

	return matches
}

// generateStopPrefix creates a stop prefix by taking the provided prefix
// and replacing its last character with the next character in the alphabet.
// If the prefix is empty, it returns an empty string.
func generateStopPrefix(prefix string) string {
	if len(prefix) == 0 {
		return ""
	}

	// Get the last character of the prefix
	lastChar := prefix[len(prefix)-1]

	// Create a new prefix by replacing the last character with its successor
	nextChar := nextLetter(lastChar)

	// Return the modified prefix
	return prefix[:len(prefix)-1] + string(nextChar)
}

// nextLetter returns the letter in the English alphabet
// that comes after the provided letter c.
func nextLetter(c byte) byte {
	switch c {
	case 'z':
		return 'a'
	case 'Z':
		return 'A'
	default:
		return c + 1
	}
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
