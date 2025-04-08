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
