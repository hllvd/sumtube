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
	"slices"
	"strconv"
	"strings"
	"time"
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
	println("langHandle path", r.URL.Path)

	// 1. Highest priority: URL path (domain.com/{lang})
	if lang, isOnPath := extractLang(r.URL.Path); isOnPath && allowedLanguages[lang] {
		setLanguageCookie(w, lang)
		println("langHandle isOnPath", lang)
		return lang
	}

	// 2. Cookie
	if cookie, err := r.Cookie("language"); err == nil {
		println("langHandle 2", cookie.Value)
		if allowedLanguages[cookie.Value] {
			println("langHandle 2 isAllowed", cookie.Value)
			return cookie.Value
		}
	}

	// 3. Accept-Language header
	if acceptLang := getBrowserLang(r); acceptLang != "" {
		println("getBrowserLang acceptLang", acceptLang)
		lang := strings.TrimSpace(strings.Split(acceptLang, ",")[0])
		if lang != "" {
			baseLang := strings.Split(lang, "-")[0]
			if allowedLanguages[baseLang] {
				println("Browser selected", baseLang)
				return baseLang
			}
		}
	}

	// 4. Default to English
	return "en"
}

// helper to set language cookie
func setLanguageCookie(w http.ResponseWriter, lang string) {
    println("setLanguageCookie : ", lang)
	cookie := &http.Cookie{
		Name:     "language",
		Value:    lang,
		Path:     "/",
		MaxAge:   86400 * 365,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}

func getBrowserLang(r *http.Request) string {
    langHeader := r.Header.Get("Accept-Language")
    if langHeader == "" {
        return "en" // default fallback
    }

    // Example header: "en-US,en;q=0.9,fr;q=0.8"
    parts := strings.Split(langHeader, ",")
    if len(parts) == 0 {
        return "en"
    }

    lang := strings.TrimSpace(parts[0]) // take the first one like "en-US"
    langCode := strings.Split(lang, "-")[0] // extract "en" from "en-US"

    return langCode
}

func extractVideoId(segments []string) string {
    // YouTube video ID pattern (11 characters of letters, numbers, underscores, or hyphens)
    idPattern := `^[A-Za-z0-9_-]{11}$`
    // Check both possible positions (0 and 1) for a valid YouTube ID
    for _, pos := range []int{0, 1} {
        if len(segments) > pos {
            candidate := segments[pos]
            if matched, _ := regexp.MatchString(idPattern, candidate); matched {
                return candidate
            }
        }
    }

    return ""
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

func extractTitle(segments []string) string {
	if len(segments) >= 3 {
		return segments[2]
	}
	return ""
}

    // ReplaceMarkdownTimestamps takes a YouTube video ID and markdown content,
// and replaces [Text](hh:mm:ss) links with [Text](https://youtu.be/VIDEO_ID?t=SECONDS)
func ReplaceMarkdownTimestamps(videoID string, content string) string {
	re := regexp.MustCompile(`\[(.*?)\]\((\d{2}):(\d{2}):(\d{2})\)`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) != 5 {
			return match // safety fallback
		}

		text := parts[1]
		hours, _ := strconv.Atoi(parts[2])
		minutes, _ := strconv.Atoi(parts[3])
		seconds, _ := strconv.Atoi(parts[4])
		totalSeconds := hours*3600 + minutes*60 + seconds

		newLink := fmt.Sprintf("[%s](https://youtu.be/%s?t=%d)", text, videoID, totalSeconds)
		return newLink
	})
}

func GetVideosFromCategory(lang string, categoryName string, limit int) ([]map[string]string, error) {
    // Monta a URL com parâmetros
    baseURL := os.Getenv("SUMTUBE_VIDEOS_RELATED_API")
    if baseURL == "" {
        return nil, fmt.Errorf("SUMTUBE_VIDEOS_RELATED_API is not set")
    }

  
    escapedCategory := url.QueryEscape(categoryName)
    fullURL := fmt.Sprintf("%s?category=%s&lang=%s&limit=%d", baseURL, escapedCategory, lang, limit)


    resp, err := http.Get(fullURL)
    if err != nil {
        return nil, fmt.Errorf("failed to call API: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API returned non-200 status: %d - %s", resp.StatusCode, string(body))
    }

    // Lê e decodifica o corpo da resposta
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %v", err)
    }

    var rawItems []map[string]interface{}
    if err := json.Unmarshal(body, &rawItems); err != nil {
        return nil, fmt.Errorf("failed to parse response JSON: %v", err)
    }

    // Extrai somente vid e title
    var videos []map[string]string
    for _, item := range rawItems {
        vid, _ := item["vid"].(string)
        title, _ := item["title"].(string)
        path, _ := item["path"].(string)
        lang, _ := item["lang"].(string)
        if vid != "" && title != "" {
            videos = append(videos, map[string]string{
                "vid":   vid,
                "title": title,
                "path":  path,
                "lang":  lang,
            })
        }
    }

    return videos, nil
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

    segments, _ := splitURLPath(path)
    videoID := extractVideoId(segments)

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
    duration := result["duration"].(int)

	htmlContent := ConvertMarkdownToHTML(content)
    
    // Create structured JSON response
    response := map[string]interface{}{
        "videoId": videoID,
        "lang":    responseLang,
        "content": htmlContent,
        "answer":  answer,
        "duration": duration,
    }
    
    // Send JSON response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func splitURLPath(rawURL string) ([]string, error) {
	// Ensure it has a scheme so url.Parse works properly
	u, err := url.ParseRequestURI("https://example.com" + rawURL)
	if err != nil {
		return nil, err
	}

	// Split path and filter out empty segments
	segments := strings.Split(u.Path, "/")
	var parts []string
	for _, segment := range segments {
		if segment != "" {
			parts = append(parts, segment)
		}
	}

	return parts, nil
}

type RouteType int

const (
	UNKNOWN RouteType = iota
	BLOG_TEMPLATE
	REDIRECT_HOME
	REDIRECT_BLOG_RETURN_HOME
	HOME
)

func (rt RouteType) String() string {
	return [...]string{"UNKNOWN", "BLOG_TEMPLATE", "REDIRECT_HOME", "REDIRECT_BLOG_HOME", "HOME"}[rt]
}

func isVideoID(s string) bool {
    // videoId do YouTube: 11 caracteres, letras, números, hífen e underscore
    var videoIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
	return videoIDPattern.MatchString(s)
}

func isLikelyTitle(s string) bool {
	return strings.Contains(s, "-") && !isVideoID(s)
}

// check if the route type based on URL
func GetRouteType(segments []string) RouteType {
	// Remove empty segments
	segments = slices.DeleteFunc(segments, func(s string) bool {
		return s == ""
	})
	n := len(segments)
    println("segments: ", segments)
    println("segments n: ", n)
    
    if n == 3 {
        first, second, third := segments[0], segments[1], segments[2]
		if allowedLanguages[first] {
			if isVideoID(second) && isLikelyTitle(third) {
                println("4 isVideoID(second) && isLikelyTitle(third)", isVideoID(second), isLikelyTitle(third))
				println("return BLOG_TEMPLATE")
                return BLOG_TEMPLATE
			}
		}
	}

    if n == 2 {
        first, second := segments[0], segments[1]
		if allowedLanguages[first] && isVideoID(second) {
            println("3 allowedLanguages[first] && isVideoID(second)", allowedLanguages[first], isVideoID(second))
            println("return REDIRECT_BLOG_RETURN_HOME")
			return REDIRECT_BLOG_RETURN_HOME
		}
	}
    
    if n == 1{
        first := segments[0]
		if allowedLanguages[first] {
            println("1 allowedLanguages: ", first)
            println("return HOME")
			return HOME
		} else if isVideoID(first) {
            println("2 isVideoID(first)", isVideoID(first))
            println("return HOME")
			return HOME
		}
	} 
    
    
    println("6 UNKNOWN")
	return REDIRECT_HOME
}

// Router function to delegate requests to the appropriate controller
func router(w http.ResponseWriter, r *http.Request) {
    pathWithParam := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "https://"), "http://")
    println("pathWithParam =>",pathWithParam)
    println("r.URL.Path ", r.URL.Path)
   // lang, _ := extractLang("en")

    if r.URL.RawQuery != "" {
        pathWithParam += "?" + r.URL.RawQuery
    }
	pathWithParamHasYoutube := strings.Contains(pathWithParam, "https://")

    pathSegments, err := splitURLPath(pathWithParam)

	if err != nil {
        http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

    routeType := GetRouteType(pathSegments)
    println("routeType: ", routeType)

    
    // Extract components
    lang, _ := extractLang(pathWithParam)
    title := extractTitle(pathSegments)
    videoId := extractVideoId(pathSegments)

	// lang = langHandle(w, r)

    switch routeType {
        case BLOG_TEMPLATE:
            println("case BLOG_TEMPLATE")
            loadBlog(w, r, lang, title, videoId)
        
        case REDIRECT_BLOG_RETURN_HOME:
            println("case REDIRECT_BLOG_RETURN_HOME")
            result, _ := GetVideoContent(videoId, lang)
            // Go do Blog
            if err == nil {
                if status, ok := result["status"].(string); ok {
                    println("here")
                    println("status: ", status)
                    if (status == "completed"){
                        path := result["path"].(string)
                        newPath := fmt.Sprintf("/%s/%s/%s", lang, videoId, path)
                        redirectURL := &url.URL{
                            Path:     newPath,
                            RawQuery: r.URL.RawQuery,
                        }
                        println("newPath",newPath)
                        http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
                    }
                }
            }
            
            println("not there")
            if (videoId != "" || (videoId != "" && lang != "")) {
                loadIndex(w, r, lang, videoId)
            }
            
            // Go to Home
            newPath := fmt.Sprintf("/%s/%s", lang, videoId)
            redirectURL := &url.URL{
                Path:     newPath,
                RawQuery: r.URL.RawQuery,
            }
            http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
        case REDIRECT_HOME:
            println("case REDIRECT_HOME")
            newPath := fmt.Sprintf("/%s", lang)
            redirectURL := &url.URL{
                Path:     newPath,
            }
            http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
        case HOME:
            println("case HOME", lang, videoId)
            loadIndex(w, r, lang, videoId)
        default:
            http.Error(w, "Invalid route", http.StatusNotFound)
	}

	println("pathWithParamHasYoutube : ", pathWithParamHasYoutube)


    // Route based on extracted components
    // switch {

	// case pathWithParamHasYoutube:
	// 	// Build the new URL with the language prefix
        
        
    //     // Perform the redirect
    //     http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
	// 	return

	// // Case 1: If lang does not exist please check the value from langHandled and redirect the user to
	// // domain.com/{lang}/the current path already introduced by the user
    // case !langOk:
    //     // Build the new URL with the language prefix
    //     newPath := fmt.Sprintf("/%s%s", langHandled, pathWithParam)
        
    //     // Create the redirect URL, preserving any query parameters
    //     redirectURL := &url.URL{
    //         Path:     newPath,
    //         RawQuery: r.URL.RawQuery,
    //     }
        
    //     // Perform the redirect
    //     http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
    //     return
	

	
	// Case 2: Only language exists - load index
    // case langOk && !titleOk && !videoOk:
	// 	println("Case 2: Only language exists - load index :",videoOk, videoId)
    //     loadIndex(w, r, lang)
        
    // // Case 3: Language, title, and video ID exist - load blog
    // case langOk && titleOk && videoOk:
    //     loadBlog(w, r, lang, title, videoId)
        
    // // Case 4: Only language and video ID exist or only videoId exist - load summary
    // case videoOk && !titleOk:
    //     loadSummary(w, r, videoId, videoId)
        
    // // Default case: Invalid route
    // default:
    //     http.Error(w, "Invalid route", http.StatusNotFound)
    // }
}


// loadIndex handles the index page
// Example URL: /en
func loadIndex(w http.ResponseWriter, r *http.Request, lang string, video ...string) {
        // Use the first video parameter if provided, otherwise use empty string
        videoId := ""
        if len(video) > 0 {
            videoId = video[0]
        }

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
            VideoId  string  // Add VideoId to the template data
            T        func(string) string // Translation function
        }{
            Language: lang,
            Path:     r.URL.Path,
            ApiUrl:   os.Getenv("SUMTUBE_API_PUBLIC"),
            VideoId:  videoId,
            T: func(key string) string {
                return t(lang, key)
            },
        }

        // Execute the template with the data
        w.Header().Set("Content-Type", "text/html")
        err = tmpl.Execute(w, data)
        if err != nil {
            http.Error(w, fmt.Sprintf("Error rendering template: %v", err), http.StatusInternalServerError)
        }
    }

    func t(lang, key string) string {
        var translations = map[string]map[string]string{
            "en": {
                "title": "YouTube Summarizer",
                "nav_login": "Login",
                "heading": "Summarize Any YouTube Video",
                "subheading": "Enter a YouTube video URL and get an AI-generated summary",
                "footer_copyright": "© 2025 YouTube Summarizer. All rights reserved.",
            },
            "pt": {
                "title": "Resumidor de vídeos do YouTube",
                "nav_login": "Entrar",
                "heading": "Resuma Qualquer Vídeo do YouTube",
                "subheading": "Digite a URL de um vídeo do YouTube e obtenha um resumo gerado por IA",
                "footer_copyright": "© 2025 Resumidor de YouTube. Todos os direitos reservados.",
            },
            "es": {
                "title": "Resumidor de videos de YouTube",
                "nav_login": "Iniciar Sesión",
                "heading": "Resume Cualquier Video de YouTube",
                "subheading": "Ingresa la URL de un video de YouTube y obtén un resumen generado por IA",
                "footer_copyright": "© 2025 Resumidor de YouTube. Todos los derechos reservados.",
            },
        }
        
        if translations[lang] == nil {
            lang = "en" // fallback to English
        }
        if val, ok := translations[lang][key]; ok {
            return val
        }
        return translations["en"][key] // fallback to English key
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

    relatedVideos, err := GetVideosFromCategory(lang, result["category"].(string), 10)

    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Extração de dados da resposta
    content := result["content"].(string)
    content = strings.ReplaceAll(content, "\\n", "\n")  // fix line breaker
    answer := result["answer"].(string)
    contentTitle := result["title"].(string)
    uploaderId := result["uploader_id"].(string)

    durationTest := result["duration"]
    println("durationTest: ", durationTest)
    durationStr := "4"
    durationInt, _ := strconv.Atoi(durationStr)


    uploadDate := ""
    if date, ok := result["video_upload_date"].(string); ok && date != "" {
        uploadDate = formatDate(lang, date)
    }

    // Duração do vídeo (string tipo "PT15M22S", "15:22" etc.)
    _, readingTimeMinutes := CountWordsAndReadingTime(content)
    timeSaved := durationInt - readingTimeMinutes
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
        RelatedVideosArr     []map[string]string
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
        Duration:             durationStr,
        VideoDurationMinutes: durationInt,
        ReadingTimeMinutes:   readingTimeMinutes,
        RelatedVideosArr:     relatedVideos,
        TimeSavedMinutes:     timeSaved,
        Content:              template.HTML(ConvertMarkdownToHTML(ReplaceMarkdownTimestamps(videoId, content))),
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