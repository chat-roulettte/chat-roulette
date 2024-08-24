package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConnectionMode_String(t *testing.T) {
	testCases := []struct {
		mode     ConnectionMode
		expected string
	}{
		{VirtualConnectionMode, "virtual"},
		{PhysicalConnectionMode, "physical"},
		{HybridConnectionMode, "hybrid"},
	}

	for _, test := range testCases {
		assert.Equal(t, test.expected, test.mode.String(), "ConnectionMode.String()")
	}
}

func TestConnectionMode_Scan(t *testing.T) {
	testCases := []struct {
		value    interface{}
		expected ConnectionMode
		isErr    bool
	}{
		{"virtual", VirtualConnectionMode, false},
		{"physical", PhysicalConnectionMode, false},
		{"hybrid", HybridConnectionMode, false},
		{"unknown", 0, true},
	}

	for _, test := range testCases {
		var mode ConnectionMode
		err := mode.Scan(test.value)
		if test.isErr {
			assert.Error(t, err, "ConnectionMode.Scan()")
		} else {
			assert.NoError(t, err, "ConnectionMode.Scan()")
			assert.Equal(t, test.expected, mode, "ConnectionMode.Scan()")
		}
	}
}

func TestConnectionMode_Value(t *testing.T) {
	testCases := []struct {
		mode     ConnectionMode
		expected string
		isErr    bool
	}{
		{PhysicalConnectionMode, "physical", false},
		{VirtualConnectionMode, "virtual", false},
		{HybridConnectionMode, "hybrid", false},
	}

	for _, test := range testCases {
		got, err := test.mode.Value()
		if test.isErr {
			assert.Error(t, err, "ConnectionMode.Value()")
		} else {
			assert.NoError(t, err, "ConnectionMode.Value()")
			assert.Equal(t, test.expected, got, "ConnectionMode.Value()")
		}
	}
}

func TestParseConnectionMode(t *testing.T) {
	testCases := []struct {
		input    string
		expected ConnectionMode
		isErr    bool
	}{
		{"unknown", 0, true},
		{"physical", PhysicalConnectionMode, false},
		{"virtual", VirtualConnectionMode, false},
		{"hybrid", HybridConnectionMode, false},
	}

	for _, test := range testCases {
		got, err := ParseConnectionMode(test.input)
		if test.isErr {
			assert.Error(t, err, "ParseConnectionMode()")
		} else {
			assert.NoError(t, err, "ParseConnectionMode()")
			assert.Equal(t, test.expected, got, "ParseConnectionMode()")
		}
	}
}
