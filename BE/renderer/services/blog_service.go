// services/blog_service.go
package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// BlogData represents the data returned by the external API.
type BlogData struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	VideoID     string `json:"videoId"`
}

// GetBlogData calls the external API and returns the blog data.
func GetBlogData(youtubeID string) (*BlogData, error) {
	// Replace this URL with the actual external API endpoint
	apiURL := fmt.Sprintf("https://api.example.com/blog?videoId=%s", youtubeID)

	// Make the HTTP GET request
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to call external API: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the JSON response
	var blogData BlogData
	if err := json.Unmarshal(body, &blogData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &blogData, nil
}