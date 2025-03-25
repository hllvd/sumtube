package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-renderer-server/controllers"
	"io"
	"net/http"
	"strings"
)

// extractYouTubeID extracts the YouTube ID from a URL or returns the input as-is.
func extractYouTubeID(input string) string {
	if strings.Contains(input, "v=") {
		// Extract the ID from a full YouTube URL
		parts := strings.Split(input, "v=")
		if len(parts) > 1 {
			return strings.Split(parts[1], "&")[0]
		}
	}
	// Assume the input is already a YouTube ID
	return input
}

// handleBlog handles the /ID route.
// func handleBlog(w http.ResponseWriter, r *http.Request) {
// 	// Extract the ID from the URL path
// 	id := strings.TrimPrefix(r.URL.Path, "/")
// 	youtubeID := extractYouTubeID(id)

// 	// Render the blog template with the YouTube ID
// 	templ.Handler(templates.BlogTemplate(youtubeID)).ServeHTTP(w, r)
// }

// LoadController handles YouTube-related requests
type LoadController struct{}

func NewLoadController() *LoadController {
	return &LoadController{}
}


func (c *LoadController) HandleLoad(w http.ResponseWriter, r *http.Request) {
	println("HandleLoad")
	// Extract the YouTube ID or URL from the path
	path := strings.TrimPrefix(r.URL.Path, "/")

	var videoID string

	// Handle YouTube ID or URL
	if strings.HasPrefix(path, "https://www.youtube.com/watch?v=") {
		// Extract the YouTube ID from the full URL
		videoID = strings.TrimPrefix(path, "https://www.youtube.com/watch?v=")
	} else {
		// Assume it's just the YouTube ID
		videoID = path
	}

	// Prepare the payload to send to the Docker API server
	payload := map[string]string{
		"videoId": videoID,
	}

	// Convert the payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
		return
	}

	// Send a POST request to the Docker API server
	resp, err := http.Post("http://go-server:8080/summary", "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		http.Error(w, "Failed to call Docker API server", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read Docker API response", http.StatusInternalServerError)
		return
	}

	// Parse the JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse Docker API response: %s", body), http.StatusInternalServerError)
		return
	}

	// Extract properties from the response
	lang := result["lang"].(string)
	content := result["content"].(string)
	answer := result["answer"].(string)

	// Respond with the extracted data
	response := fmt.Sprintf("Handling YouTube ID: %s\nLang: %s\nContent: %s\nAnswer: %s", videoID, lang, content, answer)
	w.Write([]byte(response))
}

// Router function to delegate requests to the appropriate controller
func router(w http.ResponseWriter, r *http.Request) {
	println("Entering into router")
	
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Split the path into segments
	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		http.Error(w, "Invalid URL", http.StatusNotFound)
		return
	}

	// Define valid language codes
	validLangCodes := map[string]bool{
		"es": true,
		"pt": true,
		"en": true,
	}

	// Check if the first segment is a valid language code
	firstSegment := segments[0]
	if !validLangCodes[firstSegment] {
		println("If the first segment is not a language code, assume it's a YouTube ID or URL")
		// If the first segment is not a language code, assume it's a YouTube ID or URL
		loadController := NewLoadController()
		loadController.HandleLoad(w, r)
		return
	}

	// If the first segment is a language code, check if the path contains "~"
	if strings.Contains(path, "~") {
		println("Delegate to the BlogController")
		// Delegate to the BlogController
		blogController := controllers.NewBlogController()
		blogController.HandleBlog(w, r)
		return
	}

	// Handle invalid or unknown paths
	http.Error(w, "Invalid URL", http.StatusNotFound)
}

func main() {
	// // Serve the /ID route
	// //http.HandleFunc("/", handleBlog)

	// // Create a new BlogController
	// blogController := controllers.NewBlogController()

	// // Serve the /ID route
	// http.HandleFunc("/", blogController.HandleBlog)

	// // Start the server
	// println("Server is running on http://localhost:8081")
	// http.ListenAndServe(":8081", nil)
	http.HandleFunc("/", router)

	// Start the server
	println("Server is running on http://localhost:8081 test")
	http.ListenAndServe(":8081", nil)
}