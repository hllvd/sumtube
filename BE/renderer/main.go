package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

//go:embed templates/*
var templateFS embed.FS

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

func extractLang(path string) (string, bool) {
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathParts) > 0 {
		if lang := pathParts[0]; allowedLanguages[lang] {
			return lang,true
		}
	}
	return "",false
}

func langHandle(w http.ResponseWriter, r *http.Request) string {
	// 1. Highest priority: Check URL path (domain.com/{lang})
	lang, isOnPath := extractLang(r.URL.Path)
	if isOnPath {
		// Set language cookie
        cookie := &http.Cookie{
            Name:     "language",
            Value:    lang,
            Path:     "/",
            MaxAge:   86400 * 365, // 1 year
            Secure:   true,        // Only send over HTTPS
            HttpOnly: true,        // Prevent JavaScript access
            SameSite: http.SameSiteStrictMode,
        }
        http.SetCookie(w, cookie)
		return lang
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

    // Parse the URL to handle both path and query parameters
    parsedURL, err := url.Parse(path)
    if err != nil {
        return "", false
    }

	println("decodedPathWithParameters",path)

    // Combine path and query parameters for full URL handling
    decodedPathWithParameters := parsedURL.Path
    if parsedURL.RawQuery != "" {
        decodedPathWithParameters += "?" + parsedURL.RawQuery
    }
	
    // URL decode the combined path and parameters
    decodedPathWithParameters, err = url.PathUnescape(strings.Trim(decodedPathWithParameters, "/"))
    if err != nil {
        decodedPathWithParameters = strings.Trim(path, "/")
    }

    // First try to extract from YouTube URLs
    if match := youtubeRegex.FindStringSubmatch(decodedPathWithParameters); len(match) > 1 {
        if videoIDRegex.MatchString(match[1]) {
            return match[1], true
        }
    }

    // Handle non-URL paths
    parts := strings.Split(decodedPathWithParameters, "/")
    for _, part := range parts {
        // Direct video ID (standalone or at the end of path)
        if len(part) == 11 && videoIDRegex.MatchString(part) {
            return part, true
        }
        // Video title with ID suffix (title-{videoId})
        if len(part) > 12 && strings.Contains(part, "-") {
            splitPart := strings.Split(part, "-")
            lastSegment := splitPart[len(splitPart)-1]
            if len(lastSegment) == 11 && videoIDRegex.MatchString(lastSegment) {
                return lastSegment, true
            }
        }
    }

    // Check if the last segment is a video ID in a path like /lang/title/videoId
    if len(parts) >= 3 {
        lastSegment := parts[len(parts)-1]
        if len(lastSegment) == 11 && videoIDRegex.MatchString(lastSegment) {
            return lastSegment, true
        }
    }

    return "", false
}


// ExtractYouTubeID extracts the YouTube video ID from a URL string
func extractYouTubeIFromYoutubeUrl(rawURL string) (string, bool) {
	// Handle URL-encoded strings
	decodedURL, err := url.QueryUnescape(rawURL)
	if err != nil {
		decodedURL = rawURL
	}

	// YouTube video ID pattern (11 characters of letters, numbers, underscores, or hyphens)
	idPattern := `^[A-Za-z0-9_-]{11}$`

	// First check if it's a direct ID (standalone 11 chars in YouTube format)
	if matched, _ := regexp.MatchString(idPattern, decodedURL); matched {
		return decodedURL, true
	}

	// Regular expression patterns to match YouTube URLs
	patterns := []string{
		`(?:youtube\.com\/(?:[^\/]+\/.+\/|(?:v|e(?:mbed)?)\/|.*[?&]v=)|youtu\.be\/)([A-Za-z0-9_-]{11})`, // Standard URLs
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(decodedURL)
		if len(matches) > 1 {
			return matches[1], true
		}
	}

	return "", false
}

func extractTitle(path string) (string, bool) {
	var (
		youtubeRegex = regexp.MustCompile(`(?i)(youtube\.com/watch|youtu\.be|youtube\.com/embed|youtube\.com/v|www\.youtube\.com/watch)`)
		videoIDRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_]{11}$`)
	)
		// URL decode the path first
		decodedPath, err := url.PathUnescape(strings.Trim(path, "/"))
		if err != nil {
			decodedPath = strings.Trim(path, "/")
		}
	
		// Check if this is any YouTube URL pattern
		if youtubeRegex.MatchString(decodedPath) {
			return "", false
		}
	
		parts := strings.Split(decodedPath, "/")
	
		// Case 1: /{lang}/{title}-{videoId}
		if len(parts) == 2 && strings.Contains(parts[1], "-") {
			subparts := strings.Split(parts[1], "-")
			// Check if last segment is a video ID
			if len(subparts) > 1 && videoIDRegex.MatchString(subparts[len(subparts)-1]) {
				return strings.Join(subparts[:len(subparts)-1], "-"), true
			}
			return parts[1], true
		}
	
		// Case 2: /{lang}/{title}/{videoId}
		if len(parts) >= 3 {
			title := parts[len(parts)-2]
			if title == "" {
				return "", false
			}
			return title, true
		}
	
		// Case 3: /{lang}/{title} (no video ID)
		if len(parts) == 2 {
			// Check if the second part is actually a video ID
			if videoIDRegex.MatchString(parts[1]) {
				return "", false
			}
			return parts[1], true
		}
	
		// Case 4: /{title}-{videoId} (no language)
		if len(parts) == 1 && strings.Contains(parts[0], "-") {
			subparts := strings.Split(parts[0], "-")
			// Check if last segment is a video ID
			if len(subparts) > 1 && videoIDRegex.MatchString(subparts[len(subparts)-1]) {
				return strings.Join(subparts[:len(subparts)-1], "-"), true
			}
			return parts[0], true
		}
	
		return "", false
	}

// GetVideoContent fetches content from the API for a given video ID and language
func GetVideoContent(videoID, lang string) (map[string]interface{}, error) {
    // Prepare the payload
    payload := map[string]string{
        "videoId":  videoID,
        "language": lang,
    }
    
    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("failed to encode payload: %v", err)
    }
    
    // Call the API
	//
    resp, err := http.Post(os.Getenv("SUMTUBE_API"), "application/json", bytes.NewBuffer(payloadBytes))
    if err != nil {
        return nil, fmt.Errorf("failed to call API: %v", err)
    }
    defer resp.Body.Close()
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read API response: %v", err)
    }
    
    var result map[string]interface{}
    if err := json.Unmarshal(body, &result); err != nil {
        return nil, fmt.Errorf("failed to parse API response: %v", err)
    }
    
    return result, nil
}

// ConvertMarkdownToHTML converts a markdown string to HTML
func ConvertMarkdownToHTML(md string) string {
	// Create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)

	// Parse markdown to AST
	doc := p.Parse([]byte(md))

	// Create HTML renderer with common extensions
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	// Render AST to HTML
	return string(markdown.Render(doc, renderer))
}

func parseDurationToMinutes(raw string) int {
    seconds, err := strconv.Atoi(strings.TrimSpace(raw))
    if err != nil {
        return 0
    }

    minutes := seconds / 60
    if seconds%60 > 30 {
        minutes += 1
    }
    return minutes
}


func CountWordsAndReadingTime(text string) (int, int) {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})

	wordCount := len(words)
	minutes := wordCount / 200
	if wordCount%200 > 0 {
		minutes++ // round up partial minutes
	}

	return wordCount, minutes
}


// LoadContent handles the HTTP request for video content
func (c *LoadController) LoadContent(w http.ResponseWriter, r *http.Request) {
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
    
    // Get the content using the new function
    result, err := GetVideoContent(videoID, language)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Extract properties from the response
    responseLang := result["lang"].(string)
    content := result["content"].(string)
    answer := result["answer"].(string)

	htmlContent := ConvertMarkdownToHTML(content)
    
    // Create structured JSON response
    response := map[string]interface{}{
        "videoId": videoID,
        "lang":    responseLang,
        "content": htmlContent,
        "answer":  answer,
    }
    
    // Send JSON response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Router function to delegate requests to the appropriate controller
func router(w http.ResponseWriter, r *http.Request) {
    pathWithParam := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "https://"), "http://")
    if r.URL.RawQuery != "" {
        pathWithParam += "?" + r.URL.RawQuery
    }
	pathWithParamHasYoutube := strings.Contains(pathWithParam, "https://")
    
    // Extract components
    lang, langOk := extractLang(pathWithParam)
    title, titleOk := extractTitle(pathWithParam)
    videoId, videoOk := extractVideoId(pathWithParam)

	langHandled := langHandle(w, r)


	println("pathWithParamHasYoutube : ", pathWithParamHasYoutube)


    // Route based on extracted components
    switch {

	case pathWithParamHasYoutube:
		// Build the new URL with the language prefix
        newPath := fmt.Sprintf("/%s%s", langHandled, videoId)
        
        // Create the redirect URL, preserving any query parameters
        redirectURL := &url.URL{
            Path:     newPath,
            RawQuery: r.URL.RawQuery,
        }
        
        // Perform the redirect
        http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
		return

	// Case 1: If lang does not exist please check the value from langHandled and redirect the user to
	// domain.com/{lang}/the current path already introduced by the user
    case !langOk:
        // Build the new URL with the language prefix
        newPath := fmt.Sprintf("/%s%s", langHandled, pathWithParam)
        
        // Create the redirect URL, preserving any query parameters
        redirectURL := &url.URL{
            Path:     newPath,
            RawQuery: r.URL.RawQuery,
        }
        
        // Perform the redirect
        http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
        return
	

	
	// Case 2: Only language exists - load index
    case langOk && !titleOk && !videoOk:
		println("Case 2: Only language exists - load index :",videoOk, videoId)
        loadIndex(w, r, lang)
        
    // Case 3: Language, title, and video ID exist - load blog
    case langOk && titleOk && videoOk:
        loadBlog(w, r, lang, title, videoId)
        
    // Case 4: Only language and video ID exist or only videoId exist - load summary
    case videoOk && !titleOk:
        loadSummary(w, r, videoId, videoId)
        
    // Default case: Invalid route
    default:
        http.Error(w, "Invalid route", http.StatusNotFound)
    }
}


// loadIndex handles the index page
// Example URL: /en
func loadIndex(w http.ResponseWriter, r *http.Request, lang string) {
	tmpl, err := template.ParseFS(templateFS, filepath.Join("templates", "home.html"))

	dir, _ := os.Getwd()
	fmt.Println("Current directory:", dir)
	fmt.Println("full path:", filepath.Join("templates", "home.html"))
    if err != nil {
        http.Error(w, fmt.Sprintf("Error loading template: %v", err), http.StatusInternalServerError)
        return
    }

    // Prepare data to pass to the template
    data := struct {
        Language string
        Path     string
		ApiUrl   string
    }{
        Language: lang,
        Path:     r.URL.Path,
		ApiUrl:   os.Getenv("SUMTUBE_API_PUBLIC"),
    }

    // Execute the template with the data
    w.Header().Set("Content-Type", "text/html")
    err = tmpl.Execute(w, data)
    if err != nil {
        http.Error(w, fmt.Sprintf("Error rendering template: %v", err), http.StatusInternalServerError)
    }
}

// loadBlog handles the blog page
// Example URL: /en/my-video-title/dQw4w9WgXcQ
// When parsing the template, create a FuncMap and add it to the template:
func loadBlog(w http.ResponseWriter, r *http.Request, lang, title, videoId string) {
    funcMap := template.FuncMap{
        "ConvertMarkdownToHTML": ConvertMarkdownToHTML,
    }

    tmpl := template.New("blog.html").Funcs(funcMap)
    tmpl, err := tmpl.ParseFS(templateFS, filepath.Join("templates", "blog.html"))
    if err != nil {
        http.Error(w, fmt.Sprintf("Error loading template: %v", err), http.StatusInternalServerError)
        return
    }

    result, err := GetVideoContent(videoId, lang)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Extração de dados da resposta
    content := result["content"].(string)
    answer := result["answer"].(string)
    contentTitle := result["title"].(string)
    uploaderId := result["uploader_id"].(string)

    uploadDate := ""
    if date, ok := result["upload_date"].(string); ok && date != "" {
        uploadDate = formatDate(lang, date)
    }

    // Duração do vídeo (string tipo "PT15M22S", "15:22" etc.)
    rawDuration, _ := result["duration"].(string)
    videoDurationMinutes := parseDurationToMinutes(rawDuration)
    _, readingTimeMinutes := CountWordsAndReadingTime(content)
    timeSaved := videoDurationMinutes - readingTimeMinutes
    if timeSaved < 0 {
        timeSaved = 0
    }

    data := struct {
        Language             string
        Path                 string
        ApiUrl               string
        BaseUrl              string
        VideoId              string
        Title                string
        UploadId             string
        UploadDate           string
        Duration             string
        VideoDurationMinutes int
        ReadingTimeMinutes   int
        TimeSavedMinutes     int
        Content              template.HTML
        Answer               template.HTML
    }{
        Language:             lang,
        Path:                 r.URL.Path,
        ApiUrl:               os.Getenv("SUMTUBE_API"),
        BaseUrl:              os.Getenv("BASE_URL"),
        VideoId:              videoId,
        Title:                contentTitle,
        UploadId:             uploaderId,
        UploadDate:           uploadDate,
        Duration:             rawDuration,
        VideoDurationMinutes: videoDurationMinutes,
        ReadingTimeMinutes:   readingTimeMinutes,
        TimeSavedMinutes:     timeSaved,
        Content:              template.HTML(ConvertMarkdownToHTML(content)),
        Answer:               template.HTML(ConvertMarkdownToHTML(answer)),
    }

    w.Header().Set("Content-Type", "text/html")
    err = tmpl.Execute(w, data)
    if err != nil {
        http.Error(w, fmt.Sprintf("Error rendering template: %v", err), http.StatusInternalServerError)
    }
}

// formatDate formats a date string based on language
func formatDate(lang, dateStr string) string {
	if len(dateStr) != 8 {
		return dateStr
	}

	year := dateStr[:4]
	month := dateStr[4:6]
	day := dateStr[6:8]

	monthInt, err := strconv.Atoi(month)
	if err != nil {
		return dateStr
	}

	var monthName string
	switch lang {
	case "pt":
		months := []string{"", "Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
			"Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"}
		monthName = months[monthInt]
		return fmt.Sprintf("%s de %s de %s", day, monthName, year)
	case "es":
		months := []string{"", "Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio",
			"Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"}
		monthName = months[monthInt]
		return fmt.Sprintf("%s de %s de %s", day, monthName, year)
	case "it":
		months := []string{"", "Gennaio", "Febbraio", "Marzo", "Aprile", "Maggio", "Giugno",
			"Luglio", "Agosto", "Settembre", "Ottobre", "Novembre", "Dicembre"}
		monthName = months[monthInt]
		return fmt.Sprintf("%s %s %s", day, monthName, year)
	default: // en
		months := []string{"", "January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December"}
		monthName = months[monthInt]
		return fmt.Sprintf("%s %s, %s", monthName, day, year)
	}
}


// loadSummy handles the summary page
// Example URL: /en/dQw4w9WgXcQ
func loadSummary(w http.ResponseWriter, r *http.Request, videoId string, lang string) {
    result, err := GetVideoContent(videoId, lang)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    println("HERE path: %s",  result["path"].(string) )
    // Check if path exists and redirect if it does
    if path, ok := result["path"].(string); ok && path != "" {
        baseUrl := os.Getenv("BASE_URL")
        if baseUrl == "" {
            baseUrl = "http://" + r.Host
        }
        
        // Clean up the path to remove any leading/trailing slashes
        cleanPath := strings.Trim(path, "/")
        
        // Build the new URL
        newUrl := fmt.Sprintf("%s/%s/%s/%s", baseUrl, lang, cleanPath, videoId)
        
        // Permanent redirect (301)
        http.Redirect(w, r, newUrl, http.StatusMovedPermanently)
        return
    }

    // Continue with normal template rendering if no path exists
    // Create a template function map
    funcMap := template.FuncMap{
        "ConvertMarkdownToHTML": ConvertMarkdownToHTML,
    }

    // Create a new template with the function map
    tmpl := template.New("blog.html").Funcs(funcMap)
    
    // Parse the template file
    tmpl, err = tmpl.ParseFS(templateFS, filepath.Join("templates", "blog.html"))
    if err != nil {
        http.Error(w, fmt.Sprintf("Error loading template: %v", err), http.StatusInternalServerError)
        return
    }
    
    // Extract properties from the response
    content := result["content"].(string)
    answer := result["answer"].(string)
    contentTitle := result["title"].(string)

    data := struct {
        Language    string
        Path       string
        ApiUrl     string
        BaseUrl    string
        VideoId    string
        Title      string
        Content    template.HTML
        Answer     template.HTML
    }{
        Language:    lang,
        Path:       r.URL.Path,
        ApiUrl:     os.Getenv("SUMTUBE_API"),
        BaseUrl:    os.Getenv("BASE_URL"),
        VideoId:    videoId,
        Title:      contentTitle,
        Content:    template.HTML(ConvertMarkdownToHTML(content)),
        Answer:     template.HTML(ConvertMarkdownToHTML(answer)),
    }

    // Execute the template with the data
    w.Header().Set("Content-Type", "text/html")
    err = tmpl.Execute(w, data)
    if err != nil {
        http.Error(w, fmt.Sprintf("Error rendering template: %v", err), http.StatusInternalServerError)
    }
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