package tzx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetCountriesByLetter(t *testing.T) {

	type TestCase struct {
		letter string
		count  int
	}

	testCases := []TestCase{
		{"B", 21},
		{"C", 23},
		{"Ca", 5},
		{"Cam", 2},
		{"Camb", 1},
		{"Unite", 4},
	}

	for _, tc := range testCases {
		t.Run(tc.letter, func(t *testing.T) {
			matches := GetCountriesWithPrefix(tc.letter)

			assert.Len(t, matches, tc.count)
		})
	}
}

func BenchmarkGetCountriesByLetter(b *testing.B) {
	matches := GetCountriesWithPrefix("S")

	assert.Len(b, matches, 33)
}

func Test_nextLetter(t *testing.T) {
	assert.Equal(t, "b", nextLetter("a"))
	assert.Equal(t, "e", nextLetter("d"))
	assert.Equal(t, "z", nextLetter("y"))
	assert.Equal(t, "a", nextLetter("z"))
}

func Test_GetCountryByName(t *testing.T) {
	type TestCase struct {
		name        string
		country     string
		countryCode string
		isErr       bool
	}

	testCases := []TestCase{
		{
			"success",
			"canada",
			"CA",
			false,
		},
		{
			"normalize or",
			"united states of america",
			"US",
			false,
		},
		{
			"normalize and",
			"United Kingdom of Great Britain and Northern Ireland",
			"GB",
			false,
		},
		{
			"empty",
			"",
			"",
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, ok := GetCountryByName(tc.country)

			if tc.isErr {
				assert.False(t, ok)
			} else {
				require.True(t, ok)
				assert.Equal(t, tc.countryCode, c.Code)
			}
		})
	}
}

func Test_GetAbbreviatedTimezone(t *testing.T) {
	// Phoenix, Arizona does not use daylight savings time
	// thereby simplifying our test.
	name := "America/Phoenix"

	result := GetAbbreviatedTimezone(name)

	assert.Equal(t, "MST (UTC-07:00)", result)
}
