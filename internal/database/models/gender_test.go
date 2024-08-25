package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGender_String(t *testing.T) {
	testCases := []struct {
		gender   Gender
		expected string
	}{
		{Male, "male"},
		{Female, "female"},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, tc.gender.String())
	}
}

func TestGender_Scan(t *testing.T) {
	testCases := []struct {
		value    interface{}
		expected Gender
		isErr    bool
	}{
		{"male", Male, false},
		{"female", Female, false},
		{"unknown", 0, true},
	}

	for _, test := range testCases {
		var gender Gender
		err := gender.Scan(test.value)
		if test.isErr {
			assert.Error(t, err, "Gender.Scan()")
		} else {
			assert.NoError(t, err, "Gender.Scan()")
			assert.Equal(t, test.expected, gender, "Gender.Scan()")
		}
	}
}

func TestGender_Value(t *testing.T) {
	testCases := []struct {
		gender   Gender
		expected string
		isErr    bool
	}{
		{Male, "male", false},
		{Female, "female", false},
	}

	for _, test := range testCases {
		got, err := test.gender.Value()
		if test.isErr {
			assert.Error(t, err, "Gender.Value()")
		} else {
			assert.NoError(t, err, "Gender.Value()")
			assert.Equal(t, test.expected, got, "Gender.Value()")
		}
	}
}

func TestParseGender(t *testing.T) {
	testCases := []struct {
		input    string
		expected Gender
		isErr    bool
	}{
		{"unknown", 0, true},
		{"male", Male, false},
		{"female", Female, false},
	}

	for _, test := range testCases {
		got, err := ParseGender(test.input)
		if test.isErr {
			assert.Error(t, err, "ParseGender()")
		} else {
			assert.NoError(t, err, "ParseGender()")
			assert.Equal(t, test.expected, got, "ParseGender()")
		}
	}
}
