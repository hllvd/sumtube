package main

import (
	"testing"
)

func TestExtractVideoId(t *testing.T) {
    cases := []struct {
        name     string
        path     string
        expected string
        valid    bool
    }{
        // Basic cases
        {"Simple Video ID", "dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
        {"Language with Video ID", "en/dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
        {"Language with Title and Video ID", "en/my-video-title-dQw4w9WgXcQ", "dQw4w9WgXcQ", true},

        // New test cases for /lang/title/videoId pattern
        {"Full Path with Video ID", "en/my-video-title/dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
        {"Full Path with Underscore Video ID", "pt/my-video-title/d__4w5WgXcQ", "d__4w5WgXcQ", true},
        {"French Path with Video ID", "fr/learn-french-basics/abc123def45", "abc123def45", true},
        {"Invalid Length Video ID", "fr/learn-french-basics/abc123def", "", false},

        {"Missing Video ID", "de/title-with-no-id", "", false},

        // YouTube URL cases
        {"Encoded YouTube URL", "https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3DdQw4w9WgXcQ", "dQw4w9WgXcQ", true},
        {"Encoded YouTube Short URL", "youtu.be%2FdQw4w9WgXcQ", "dQw4w9WgXcQ", true},
        {"Encoded YouTube URL with Parameters", "https%3A%2F%2Fyoutu.be%2FdQw4w9WgXcQ%3Fsi%3DvaigGpt3EpD6fA84", "dQw4w9WgXcQ", true},
        {"Encoded WWW YouTube URL", "www.youtube.com%2Fwatch%3Fv%3DdQw4w9WgXcQ", "dQw4w9WgXcQ", true},
        {"Encoded Basic YouTube URL", "youtube.com%2Fwatch%3Fv%3DdQw4w9WgXcQ", "dQw4w9WgXcQ", true},
        {"Encoded YouTube Embed URL", "youtube.com%2Fembed%2FdQw4w9WgXcQ", "dQw4w9WgXcQ", true},
        {"YouTube Short URL", "youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
        {"YouTube Embed URL", "youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ", true},

        {"YouTube Url with lang", "en%2Fhttps%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3DdQw4w9WgXc3", "dQw4w9WgXc3", true},

        // Invalid cases
        {"Invalid Input", "invalid", "", false},
        {"Missing Video ID in Title", "en/my-video-title", "", false},
        {"Invalid YouTube URL", "youtube.com/watch", "", false},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            videoID, ok := extractVideoId(tc.path)

            if ok != tc.valid {
                t.Errorf("Expected valid: %v, got: %v", tc.valid, ok)
            }

            if ok && videoID != tc.expected {
                t.Errorf("Expected video ID: %s, got: %s", tc.expected, videoID)
            }
        })
    }
}

func TestExtractTitle(t *testing.T) {
    cases := []struct {
        name     string
        path     string
        expected string
        valid    bool
    }{
        // Basic cases
        {"Video ID Only", "dQw4w9WgXcQ", "", false},
        {"Language with Video ID", "en/dQw4w9WgXcQ", "", false},
        {"Language with Title and Video ID", "en/my-video-title-dQw4w9WgXcQ", "my-video-title", true},

        // New test cases for /lang/title/videoId pattern
        {"Full Path with Title", "en/my-video-title/dQw4w9WgXcQ", "my-video-title", true},
        {"Portuguese Path with Title", "pt/my-video-title/d__4w5WgXcQ", "my-video-title", true},
        {"French Path with Title", "fr/learn-french-basics/abc123def45", "learn-french-basics", true},
        {"French Path with Invalid Video ID", "fr/learn-french-basics/abc123def", "learn-french-basics", true},

        {"Title with No Video ID", "de/title-with-no-id", "title-with-no-id", true},

        // YouTube URL cases
        {"Encoded YouTube URL", "https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3DdQw4w9WgXcQ", "", false},
        {"Encoded YouTube Short URL", "youtu.be%2FdQw4w9WgXcQ", "", false},
        {"Encoded YouTube URL with Parameters", "https%3A%2F%2Fyoutu.be%2FdQw4w9WgXcQ%3Fsi%3DvaigGpt3EpD6fA84", "", false},
        {"Encoded WWW YouTube URL", "www.youtube.com%2Fwatch%3Fv%3DdQw4w9WgXcQ", "", false},
        {"Encoded Basic YouTube URL", "youtube.com%2Fwatch%3Fv%3DdQw4w9WgXcQ", "", false},
        {"Encoded YouTube Embed URL", "youtube.com%2Fembed%2FdQw4w9WgXcQ", "", false},
        {"YouTube Short URL", "youtu.be/dQw4w9WgXcQ", "", false},
        {"YouTube Embed URL", "youtube.com/embed/dQw4w9WgXcQ", "", false},

        // Invalid cases
        {"Invalid Input", "invalid", "", false},
        {"Simple Title", "en/my-video-title", "my-video-title", true},
        {"Invalid YouTube Watch URL", "youtube.com/watch", "", false},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            title, ok := extractTitle(tc.path)

            if ok != tc.valid {
                t.Errorf("path %q - expected valid: %v, got: %v", tc.path, tc.valid, ok)
            }

            if ok && title != tc.expected {
                t.Errorf("path %q - expected title: %q, got: %q", tc.path, tc.expected, title)
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

