package main

import (
	"strings"
	"testing"
)

func TestExtractVideoId(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
		valid    bool
	}{
		// Basic cases
		{"Simple Video ID", "dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
		{"Language with Video ID", "en/dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
		{"Language with Title and Video ID", "en/my-video-title-dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
		{"Full Path with Video ID", "en/my-video-title/dQw4w9WgXcQ", "dQw4w9WgXcQ", true},

		// YouTube URL cases
		{"Encoded YouTube URL", "https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3DdQw4w9WgXcQ", "dQw4w9WgXcQ", true},
		{"Encoded YouTube Short URL", "youtu.be%2FdQw4w9WgXcQ", "dQw4w9WgXcQ", true},
		{"Encoded YouTube URL with Params", "https%3A%2F%2Fyoutu.be%2FdQw4w9WgXcQ%3Fsi%3Dabc123", "dQw4w9WgXcQ", true},
		{"YouTube Short URL", "youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
		{"YouTube Embed URL", "youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
		{"YouTube Watch URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ", true},

		// Mixed path and URL
		{"YouTube URL with lang prefix", "en%2Fhttps%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3DdQw4w9WgXc3", "dQw4w9WgXc3", true},

		// Invalid or incomplete inputs
		{"Invalid Length Video ID", "fr/learn-french-basics/abc123def", "", false},
		{"Missing Video ID", "de/title-with-no-id", "", false},
		{"Invalid Input", "invalid", "", false},
		{"Invalid YouTube URL", "youtube.com/watch", "", false},
		{"Incomplete YouTube URL", "https://www.youtube.com/watch?v=", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			segments := strings.Split(tc.input, "/")
			got := extractVideoId(segments)

			if got != tc.expected {
				t.Errorf("Input: %s | Expected: %s | Got: %s", tc.input, tc.expected, got)
			}
		})
	}
}

func TestExtractTitle(t *testing.T) {
	testCases := []struct {
		name     string
		segments []string
		expected string
	}{
		{
			name:     "Valid with more than 3 elements",
			segments: []string{"one", "two", "MyTitle", "four"},
			expected: "MyTitle",
		},
		{
			name:     "Exactly 3 elements",
			segments: []string{"first", "second", "TitleHere"},
			expected: "TitleHere",
		},
		{
			name:     "Less than 3 elements - empty",
			segments: []string{},
			expected: "",
		},
		{
			name:     "Less than 3 elements - one item",
			segments: []string{"onlyone"},
			expected: "",
		},
		{
			name:     "Less than 3 elements - two items",
			segments: []string{"first", "second"},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractTitle(tc.segments)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestExtractYouTubeIFromYoutubeUrl(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantID   string
		wantFound bool
	}{
		{
			name:     "Standard watch URL",
			input:    "https://www.youtube.com/watch?v=ZQY_RsXmpzU",
			wantID:   "ZQY_RsXmpzU",
			wantFound: true,
		},
		{
			name:     "Short URL",
			input:    "https://youtu.be/AnLTl5fQWV0?si=qm51HmyWoqPsnaX6",
			wantID:   "AnLTl5fQWV0",
			wantFound: true,
		},
		{
			name:     "URL-encoded embed",
			input:    "youtube.com%2Fembed%2FdQw4w9WgXcQ",
			wantID:   "dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			name:     "Direct ID",
			input:    "dQw4w9WgXcQ",
			wantID:   "dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			name:     "Invalid URL",
			input:    "invalid.url",
			wantID:   "",
			wantFound: false,
		},
		{
			name:     "Empty string",
			input:    "",
			wantID:   "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotFound := extractYouTubeIFromYoutubeUrl(tt.input)
			if gotID != tt.wantID {
				t.Errorf("extractYouTubeIFromYoutubeUrl() gotID = %v, want %v", gotID, tt.wantID)
			}
			if gotFound != tt.wantFound {
				t.Errorf("extractYouTubeIFromYoutubeUrl() gotFound = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

