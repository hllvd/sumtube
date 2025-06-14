package main

import (
	"testing"
)

func TestExtractLastTimestamp(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name: "Single timestamp line",
			input: `0:18:32,000 --> 0:18:38,000`,
			expected: "0:18:38",
			wantErr:  false,
		},
		{
			name: "Multiple lines",
			input: `1
0:00:01,000 --> 0:00:04,000
Hello

2
0:00:05,000 --> 0:00:09,000
World

3
0:00:09,000 --> 0:00:15,000
Test

4
0:00:15,000 --> 0:00:20,000
Final line`,
			expected: "0:00:20",
			wantErr:  false,
		},
		{
			name: "Realistic SRT example",
			input: `454
0:18:24,000 --> 0:18:28,000
Some text

455
0:18:26,000 --> 0:18:32,000
More text

456
0:18:28,000 --> 0:18:32,000
Another text

457
0:18:32,000 --> 0:18:38,000
End line`,
			expected: "0:18:38",
			wantErr:  false,
		},
		{
			name:    "No timestamps",
			input:   `This is just text without timestamps.`,
			expected: "",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ExtractLastTimestamp(tc.input)

			if (err != nil) != tc.wantErr {
				t.Errorf("Expected error: %v, got error: %v", tc.wantErr, err)
				return
			}

			if result != tc.expected {
				t.Errorf("Expected result: %s, got: %s", tc.expected, result)
			}
		})
	}
}
