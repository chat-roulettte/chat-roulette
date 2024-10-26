package templatex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

func Test_Capitalize(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"weekday", "friday", "Friday"},
		{"city", "New york", "New York"},
		{"city", "new York", "New York"},
		{"city", "ABU DHABI", "Abu Dhabi"},
		{"country", "united states of america", "United States Of America"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, Capitalize(tc.input))
		})
	}
}

func Test_CapitalizeInterval(t *testing.T) {
	actual := CapitalizeInterval(models.Biweekly)
	expected := "Biweekly"

	assert.Equal(t, expected, actual)
}

func Test_PrettyDate(t *testing.T) {
	date := time.Date(2021, time.January, 4, 12, 0, 0, 0, time.UTC)

	actual := PrettyDate(date)
	expected := "January 4th, 2021"

	assert.Equal(t, expected, actual)
}

func Test_PrettierDate(t *testing.T) {
	date := time.Date(2021, time.January, 4, 12, 0, 0, 0, time.UTC)

	actual := PrettierDate(date)
	expected := "Monday, January 4th, 2021"

	assert.Equal(t, expected, actual)
}

func Test_PrettyURL(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"http", "http://www.example.com/path/to/resource", "www.example.com/path/to/resource"},
		{"with query param", "https://www.example.com/path/to/resource?query=param", "www.example.com/path/to/resource?query=param"},
		{"with port", "http://www.example.com:8080/path", "www.example.com:8080/path"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, PrettyURL(tc.input))
		})
	}
}

func Test_PrettyPercent(t *testing.T) {
	testCases := []struct {
		name     string
		input    float64
		expected string
	}{
		{"no decimals", 100.0, "100%"},
		{"decimals", 66.66666666, "66.67%"},
		{"one third", 33.3333, "33.33%"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, PrettyPercent(tc.input))
		})
	}
}

func Test_DerefBool(t *testing.T) {
	b := true

	actual := DerefBool(&b)
	expected := true

	assert.Equal(t, expected, actual)
}
