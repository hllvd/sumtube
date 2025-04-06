package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-renderer-server/controllers"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// LoadController handles YouTube-related requests
type LoadController struct{}

func NewLoadController() *LoadController {
	return &LoadController{}
}

var allowedLanguages = map[string]bool{
	"en": true,
	"pt": true,
	"es": true,
	"it": true,
	"fr": true,
	"de": true,
}

func langHandle(w http.ResponseWriter, r *http.Request) string {
	// 1. Highest priority: Check URL path (domain.com/{lang})
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) > 0 {
		if lang := pathParts[0]; allowedLanguages[lang] {
			return lang
		}
	}

	// 2. Check language cookie
	if cookie, err := r.Cookie("language"); err == nil {
		if allowedLanguages[cookie.Value] {
			return cookie.Value
		}
	}

	// 3. Check browser Accept-Language header
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		// Parse the first language in the header (e.g., "en-US,en;q=0.9" -> "en")
		if lang := strings.Split(acceptLang, ",")[0]; lang != "" {
			// Extract base language code (en-US -> en)
			baseLang := strings.Split(lang, "-")[0]
			if allowedLanguages[baseLang] {
				return baseLang
			}
		}
	}

	// 4. Default to English
	return "en"
}


func extractVideoId(path string) (string, bool) {
	var (
		videoIDRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_]{11}$`)
		youtubeRegex = regexp.MustCompile(`(?i)(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/embed/|youtube\.com/v/|www\.youtube\.com/watch\?v=)([a-zA-Z0-9\-_]{11})`)
	)
	// URL decode the path first
	decodedPath, err := url.PathUnescape(strings.Trim(path, "/"))
	if err != nil {
		decodedPath = strings.Trim(path, "/")
	}

	// First try to extract from YouTube URLs
	if match := youtubeRegex.FindStringSubmatch(decodedPath); len(match) > 1 {
		if videoIDRegex.MatchString(match[1]) {
			return match[1], true
		}
	}

	// Handle non-URL paths
	parts := strings.Split(decodedPath, "/")
	for _, part := range parts {
		// Direct video ID
		if len(part) == 11 && videoIDRegex.MatchString(part) {
			return part, true
		}

		// Video title with ID suffix
		if len(part) > 12 && strings.Contains(part, "-") {
			splitPart := strings.Split(part, "-")
			lastSegment := splitPart[len(splitPart)-1]
			if len(lastSegment) == 11 && videoIDRegex.MatchString(lastSegment) {
				return lastSegment, true
			}
		}
	}

	return "", false
}


func (c *LoadController) HandleLoad(w http.ResponseWriter, r *http.Request) {
    println("HandleLoad")
	path := r.URL.Path
    
	// Extract user's lang
	language := langHandle(w, r)

	videoID, isVideoID := extractVideoId(path)
    if !isVideoID {
        http.Error(w, "Invalid YouTube ID", http.StatusBadRequest)
        return
    }

	println("HandleLoad videoID: ", videoID)
	println("HandleLoad language: ", language)
	
    // Prepare the payload to send to the Docker API server
    payload := map[string]string{
        "videoId":  videoID,
        "language": language,
    }
    
    // Convert the payload to JSON
    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        http.Error(w, "Failed to encode payload", http.StatusInternalServerError)
        return
    }
    
    // Send a POST request to the Docker API server
    resp, err := http.Post(os.Getenv("SUMTUBE_API"), "application/json", bytes.NewBuffer(payloadBytes))
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
    responseLang := result["lang"].(string)
    content := result["content"].(string)
    answer := result["answer"].(string)
    
    // Create structured JSON response
    response := map[string]interface{}{
        "videoId": videoID,
        "lang":    responseLang,
        "content": content,
        "answer":  answer,
    }
    
    // Send JSON response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
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

	// Check if the first segment is a valid language code
	firstSegment := segments[0]
	if !allowedLanguages[firstSegment] {
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