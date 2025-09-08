package main

import (
	"testing"
)

func TestGetRouteType(t *testing.T) {
	// Mock allowedLanguages


	tests := []struct {
		name     string
		segments []string
		want     RouteType
	}{
		{
			name:     "Home EN",
			segments: []string{"en"},
			want:     HOME,
		},
		{
			name:     "Home PT",
			segments: []string{"pt"},
			want:     HOME,
		},
		{
			name:     "Empty domain root",
			segments: []string{},
			want:     REDIRECT_HOME,
		},
		{
			name:     "Trailing slash PT",
			segments: []string{"pt", ""},
			want:     HOME, // should normalize and treat like "pt"
		},
		{
			name:     "Invalid language",
			segments: []string{"hashodmain", "pt"},
			want:     REDIRECT_HOME,
		},
		{
			name:     "Invalid language with trailing slash",
			segments: []string{"hashodmain", "pt", ""},
			want:     REDIRECT_HOME,
		},
		{
			name:     "Lang + VideoID → Redirect to blog",
			segments: []string{"en", "abcdefghijk"},
			want:     REDIRECT_BLOG_RETURN_HOME,
		},
		{
			name:     "Lang + VideoID + Title → Blog template",
			segments: []string{"en", "abcdefghijk", "my-title"},
			want:     BLOG_TEMPLATE,
		},
		{
			name:     "VideoID only → Home",
			segments: []string{"abcdefghijk"},
			want:     HOME,
		},
		{
			name:     "Garbage route",
			segments: []string{"foo", "bar"},
			want:     REDIRECT_HOME,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRouteType(tt.segments)
			if got != tt.want {
				t.Errorf("GetRouteType(%v) = %v, want %v", tt.segments, got, tt.want)
			}
		})
	}
}

func TestExtractLang(t *testing.T) {

	tests := []struct {
		name     string
		input    string
		wantLang string
		wantOk   bool
	}{
		{"Valid language at start", "en/video-title", "en", true},
		{"Valid lang only", "en", "en", true},
		{"Valid lang only", "/en", "en", true},
		{"Valid lang only", "/en/", "en", true},
		{"Valid language with leading slash", "/en/video-title", "en", true},
		{"Valid language with trailing slash", "en/video-title/", "en", true},
		{"Valid language with both slashes", "/en/video-title/", "en", true},
		{"Multiple segments with valid language", "fr/my/video/path", "fr", true},
		{"Invalid language", "xx/video-title", "", false},
		{"Empty path", "", "", false},
		{"Single segment invalid", "invalid", "", false},
		{"Single segment valid", "es", "es", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLang, gotOk := extractLang(tt.input)
			if gotLang != tt.wantLang {
				t.Errorf("extractLang() gotLang = %v, want %v", gotLang, tt.wantLang)
			}
			if gotOk != tt.wantOk {
				t.Errorf("extractLang() gotOk = %v, want %v", gotOk, tt.wantOk)
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

