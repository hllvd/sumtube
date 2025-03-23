package main

import (
	"net/http"
	"strings"

	"go-renderer-server/controllers"
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

func main() {
	// Serve the /ID route
	//http.HandleFunc("/", handleBlog)

	// Create a new BlogController
	blogController := controllers.NewBlogController()

	// Serve the /ID route
	http.HandleFunc("/", blogController.HandleBlog)

	// Start the server
	println("Server is running on http://localhost:8081")
	http.ListenAndServe(":8081", nil)
}