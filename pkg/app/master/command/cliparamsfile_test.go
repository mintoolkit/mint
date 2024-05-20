package command

import (
	"os"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestParseJsonFileParamsJSONUnmarshals(t *testing.T) {
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "example.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up temp file

	// Write JSON content to the temporary file
	jsonContent := `{
		"key1": "value1",
		"key2": 2,
		"key3": true
	}`

	if _, err := tmpfile.Write([]byte(jsonContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Call the function with the temporary file path
	params, err := parseJsonFileParams(tmpfile.Name())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Validate the results
	expected := map[string]interface{}{
		"key1": "value1",
		"key2": float64(2), // JSON unmarshals numbers into float64 by default
		"key3": true,
	}
	for key, expectedValue := range expected {
		if value, exists := params[key]; !exists || value != expectedValue {
			t.Errorf("For key %q, expected %v, got %v", key, expectedValue, value)
		}
	}
}

func TestParseJsonFileParamsFileNotExist(t *testing.T) {
	_, err := parseJsonFileParams("file-that-does-not-exist.json")
	if err == nil {
		t.Fatalf("Expected an error for non-existent file, but got none")
	}

	if _, ok := err.(*os.PathError); !ok {
		t.Fatalf("Expected a *os.PathError, but got %T", err)
	}
}

func TestFlagSetStringExpectedStringResult(t *testing.T) {
	tests := []struct {
		name          string
		flag          cli.Flag
		paramValue    interface{}
		expectedValue string
	}{
		{
			name:          "StringFlag with string value",
			flag:          &cli.StringFlag{Name: "test"},
			paramValue:    "value",
			expectedValue: "value",
		},
		{
			name:          "StringSliceFlag with slice value",
			flag:          &cli.StringSliceFlag{Name: "test"},
			paramValue:    []string{"value1", "value2"},
			expectedValue: "value1,value2",
		},
		{
			name:          "BoolFlag with bool value",
			flag:          &cli.BoolFlag{Name: "test"},
			paramValue:    true,
			expectedValue: "true",
		},
		{
			name:          "IntFlag with int value",
			flag:          &cli.IntFlag{Name: "test"},
			paramValue:    44,
			expectedValue: "44",
		},
		{
			name:          "IntFlag with float64 value",
			flag:          &cli.IntFlag{Name: "test"},
			paramValue:    float64(10),
			expectedValue: "10",
		},
		{
			name:          "StringSliceFlag with string value",
			flag:          &cli.StringSliceFlag{Name: "test"},
			paramValue:    "value",
			expectedValue: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := flagSetString(tt.flag, tt.paramValue)

			if err != nil {
				t.Fatalf("Expected no error, but got %v", err)
			}

			if result != tt.expectedValue {
				t.Fatalf("Expected result %q, but got %q", tt.expectedValue, result)
			}
		})
	}
}

type EmptyStruct struct{}

func TestFlagSetStringInvalidFlagValues(t *testing.T) {
	tests := []struct {
		name        string
		flag        cli.Flag
		paramValue  interface{}
		expectedErr string
	}{
		{
			name:        "StringSliceFlag with bool value",
			flag:        &cli.StringSliceFlag{Name: "test"},
			paramValue:  true,
			expectedErr: "expected string or string array value found: bool for flag test",
		},
		{
			name:        "StringSliceFlag with bool value",
			flag:        &cli.StringSliceFlag{Name: "test"},
			paramValue:  84,
			expectedErr: "expected string or string array value found: int for flag test",
		},
		{
			name:        "StringSliceFlag with bool value",
			flag:        &cli.StringSliceFlag{Name: "test"},
			paramValue:  EmptyStruct{},
			expectedErr: "expected string or string array value found: command.EmptyStruct for flag test",
		},
		{
			name:        "BoolFlag with string value",
			flag:        &cli.BoolFlag{Name: "test"},
			paramValue:  "true",
			expectedErr: "expected boolean value found: string for flag test",
		},
		{
			name:        "BoolFlag with int value",
			flag:        &cli.BoolFlag{Name: "test"},
			paramValue:  41,
			expectedErr: "expected boolean value found: int for flag test",
		},
		{
			name:        "StringFlag with slice value",
			flag:        &cli.StringFlag{Name: "test"},
			paramValue:  []string{"value1", "value2"},
			expectedErr: "expected string value found: []string for flag test",
		},
		{
			name:        "StringFlag with int value",
			flag:        &cli.StringFlag{Name: "test"},
			paramValue:  4,
			expectedErr: "expected string value found: int for flag test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := flagSetString(tt.flag, tt.paramValue)

			if err == nil || err.Error() != tt.expectedErr {
				t.Fatalf("Expected error %q, but got %q", tt.expectedErr, err)
			}
		})
	}
}

func TestFlagSetStringUnsupportedFlagError(t *testing.T) {
	tests := []struct {
		name       string
		flag       cli.Flag
		paramValue interface{}
	}{
		{
			name:       "StringSliceFlag with bool value",
			flag:       &cli.GenericFlag{Name: "test"},
			paramValue: EmptyStruct{},
		},
		{
			name:       "StringSliceFlag with bool value",
			flag:       &cli.UintFlag{Name: "test"},
			paramValue: EmptyStruct{},
		},
		{
			name:       "StringSliceFlag with bool value",
			flag:       &cli.DurationFlag{Name: "test"},
			paramValue: EmptyStruct{},
		},
	}

	const expectedError = "flag type not supported by params file"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := flagSetString(tt.flag, tt.paramValue)

			if err == nil || err.Error() != expectedError {
				t.Fatalf("Expected error %q, but got %q", expectedError, err)
			}
		})
	}
}
