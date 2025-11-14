package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"context"

	_ "embed"

	"my_lambda_app/videostate"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rs/cors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

//go:embed prompts/system1.prompt.txt
var system1PromptTxt string

//go:embed prompts/system2.prompt.txt
var system2PromptTxt string

//go:embed prompts/user1.prompt.txt
var user1PromptTxt string

var videoQueue = videostate.NewProcessor()

var processingVideosOld = make(map[string]bool)
var processingMu sync.Mutex

// OAuth2 configuration for Google
var googleOAuthConfig = &oauth2.Config{
	ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),     // Set your Google OAuth Client ID
	ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"), // Set your Google OAuth Client Secret
	RedirectURL:  "https://b0fe-2804-14c-cc92-94de-a04e-df67-2d58-c638.ngrok-free.app/redirects",  // Redirect URL must match the one configured in Google Console
	Scopes:       []string  { 
                                "https://www.googleapis.com/auth/userinfo.email", 
                                "https://www.googleapis.com/auth/youtube.download",
                                "https://www.googleapis.com/auth/youtube.force-ssl",
                                "https://www.googleapis.com/auth/youtubepartner",
                            },
	Endpoint:     google.Endpoint,
}

const tempDir = "/tmp"
const bucketName = "sumtube"
var dynamoDBTableName = os.Getenv("DYNAMODB_TABLE_NAME")



var s3Client *s3.Client
var dynamoDBClient *dynamodb.Client

func init() {
	fmt.Println("Initializing AWS SDK...")
	if dynamoDBTableName == "" {
		log.Println("Please add DYNAMODB_TABLE_NAME to .env file")
	}
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		log.Fatalf("unable to load AWS SDK config, %v\n", err)
	}
	s3Client = s3.NewFromConfig(cfg)
	dynamoDBClient = dynamodb.NewFromConfig(cfg)
	fmt.Println("AWS SDK initialized successfully.")
}

func extractVideoID(url string) (string, error) {
	re := regexp.MustCompile(`(?:v=|\/)([0-9A-Za-z_\-‑]{11}).*`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("no video ID found in URL: %s", url)
	}
	return matches[1], nil
}

type Caption struct {
	BaseURL string `json:"base_url"`
	Lang    string `json:"lang"`
}

type VideoMetadata struct {
	Title        string    `json:"title"`
	ViewCount    string    `json:"view_count"`
	LengthSeconds string   `json:"length_seconds"`
	ChannelId    string    `json:"channel_id"`
	ChannelName  string    `json:"channel_name"`
	ChannelURL   string    `json:"channel_url"`
	PublishDate  string    `json:"publish_date"`
	Category     string    `json:"category"`
	Captions     []Caption `json:"captions"`
}



func getVideoMetadata(videoURL string) (*VideoMetadata, error) {
	// proxy := os.Getenv("YDT_PROXY_SERVER")
	
	videoID, err := extractVideoID(videoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract video ID: %w", err)
	}

	baseURL := "http://youtube-metadata-server:6060/metadata"
	query := url.Values{}
	query.Set("vid", videoID)


	fullURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())

	// Create a POST request with no body (nil), but with query parameters
	req, err := http.NewRequest("POST", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send request using default HTTP client
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request subtitle: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subtitle server returned status %d", resp.StatusCode)
	}

	// Read subtitle data
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read subtitle data: %w", err)
	}

	var metadata VideoMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	fmt.Print("metadata", metadata)
	return &metadata, nil
}



func removeDuplicateHyphens(s string) string {
    // Create a new strings.Builder for efficient string concatenation
    var result strings.Builder
    var lastChar rune
    
    for i, char := range s {
        // Skip this character if it's a hyphen and either:
        // 1. It's not the first character and the previous character was a hyphen
        // 2. It's the last character
        if char == '-' {
            if (i > 0 && lastChar == '-') || i == len(s)-1 {
                continue
            }
        }
        result.WriteRune(char)
        lastChar = char
    }
    
    return result.String()
}

func sanitizeTitle(title string) string {
    // Create a strings.Builder for efficient string concatenation
    var result strings.Builder
    
    // Keep track of last character to handle duplicates
    var lastChar rune
    var isFirstChar = true
    
    // Convert to lowercase and trim spaces
    title = strings.TrimSpace(strings.ToLower(title))
    
    for _, char := range title {
        // Skip if current character is a space, hyphen, or slash AND
        // it's the same as the last character we wrote
        if (unicode.IsSpace(char) || char == '-' || char == '/') {
            // Convert all separators (spaces, hyphens, slashes) to single space
            if !isFirstChar && lastChar != ' ' {
                result.WriteRune(' ')
                lastChar = ' '
            }
            continue
        }
        
        // Write character if it's not a duplicate separator
        result.WriteRune(char)
        lastChar = char
        isFirstChar = false
    }
    
    // Convert result to string and trim any remaining spaces
    sanitized := strings.TrimSpace(result.String())
    
    return sanitized
}


func convertTitleToURL(title string) string {
	lowercaseTitle := strings.ToLower(title)
	urlFriendly := strings.ReplaceAll(lowercaseTitle, " ", "-")
	urlFriendly = removeDuplicateHyphens(urlFriendly)
	var result strings.Builder
	for _, char := range urlFriendly {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func downloadSubtitleByDownSub(downSubUrl string) (string, error) {
	//capUrl := metadataResponse.Captions[0].BaseURL

	client := &http.Client{
		Timeout: 10 * time.Second, // Total timeout for the request, including connection + reading body
	}

	resp, err := client.Get(downSubUrl)
	if err != nil {
		return "", fmt.Errorf("failed to download subtitle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("subtitle server returned status %d", resp.StatusCode)
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle data: %w", err)
	}

    return string(content), nil

}

func downloadSubtitle(videoURL string, lang string) (string, error) {
	videoID, err := extractVideoID(videoURL)
	if err != nil {
		return "", fmt.Errorf("failed to extract video ID: %w", err)
	}

	outputTemplate := filepath.Join(tempDir, "%s.srt") // Template for saving file

	// Try to download in the requested language
	fmt.Printf("Attempting to download subtitles in language '%s'...\n", lang)
	subtitle, err := tryDownloadSubtitle(videoURL, videoID, outputTemplate, lang)
	if err == nil {
		return subtitle, nil
	}

	// If failed, try with English
	if lang != "en" {
		fmt.Printf("Failed to download subtitles in language '%s', trying English instead...\n", lang)
		subtitle, err = tryDownloadSubtitle(videoURL, videoID, outputTemplate, "en")
		if err == nil {
			return subtitle, nil
		}
	}

	// If both attempts fail, return error
	return "", fmt.Errorf("failed to download subtitles in either '%s' or 'en': %w", lang, err)
}

// tryDownloadSubtitle requests subtitles from the internal transcript server and saves them to disk.

func tryDownloadSubtitle(videoURL, videoID, outputTemplate, lang string) (string, error) {
	baseURL := "http://youtube-transcript-py-server:5050/transcript"
	query := url.Values{}
	query.Set("vid", videoID)
	query.Set("lang", lang)
	query.Set("format", "srt")

	fullURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())

	// Create a POST request with no body (nil), but with query parameters
	req, err := http.NewRequest("POST", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Send request using default HTTP client
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request subtitle: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("subtitle server returned status %d", resp.StatusCode)
	}

	// Read subtitle data
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle data: %w", err)
	}

	// Save to file
	subtitlePath := filepath.Join(tempDir, fmt.Sprintf("%s.%s.srt", videoID, lang))
	if err := ioutil.WriteFile(subtitlePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write subtitle file: %w", err)
	}

	return string(data), nil
}

func tryDownloadSubtitle_yt_dlp(videoURL, videoID, outputTemplate, lang string) (string, error) {
	// Get proxy from environment
	proxy := os.Getenv("YDT_PROXY_SERVER")

	// Build yt-dlp arguments
	args := []string{
		"--skip-download",
		"--write-auto-sub",
		"--sub-lang", lang,
		"--convert-subs", "srt",
		"-k",
		"-o", outputTemplate,
	}
	if proxy != "" {
		args = append(args, "--proxy", proxy)
	}
	args = append(args, videoURL)

	cmd := exec.Command("yt-dlp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run yt-dlp: %w", err)
	}

	searchQuery := fmt.Sprintf("%s*.vtt", videoID)
	subtitleFilePath := filepath.Join(tempDir, searchQuery)
	files, err := filepath.Glob(subtitleFilePath)
	if err != nil {
		return "", fmt.Errorf("error finding subtitle file: %w", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no subtitle files found")
	}

	subtitleFilePath = files[0]
	data, err := ioutil.ReadFile(subtitleFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle file: %w", err)
	}

	return string(data), nil
}

func uploadToS3(content string, key string) error {
	fmt.Println("Uploading subtitle to S3...")
	_, err := s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        strings.NewReader(content),
		ContentType: aws.String("text/plain"),
		ACL:         types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}
	fmt.Println("Subtitle uploaded to S3 successfully.")
	return nil
}

func fetchS3(key string) (string, error) {
    fmt.Println("Fetching content from S3...")
    
    // Get the object from S3
    output, err := s3Client.GetObject(context.Background(), &s3.GetObjectInput{
        Bucket: aws.String(bucketName),
        Key:    aws.String(key),
    })
    if err != nil {
        return "", fmt.Errorf("failed to fetch from S3: %w", err)
    }
    defer output.Body.Close()

    // Read the content from the response body
    content, err := io.ReadAll(output.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read S3 content: %w", err)
    }

    fmt.Println("Content fetched from S3 successfully.")
    return string(content), nil
}




func stringMapToAttributeValueMap(m map[string]string) map[string]dynamodbtypes.AttributeValue {
    avMap := make(map[string]dynamodbtypes.AttributeValue)
    for k, v := range m {
        avMap[k] = &dynamodbtypes.AttributeValueMemberS{Value: v}
    }
    return avMap
}

func deleteVideoFromDynamoDB(videoId string) error {
	key := map[string]dynamodbtypes.AttributeValue{
		"PK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("VIDEO#%s", videoId)},
		"SK": &dynamodbtypes.AttributeValueMemberS{Value: "METADATA"},
	}

	_, err := dynamoDBClient.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: aws.String(dynamoDBTableName),
		Key:       key,
	})
	if err != nil {
		return fmt.Errorf("failed to delete video %s from DynamoDB: %w", videoId, err)
	}

	fmt.Printf("Video %s deleted from DynamoDB successfully.\n", videoId)
	return nil
}

func pushMetadataToDynamoDB(data videostate.Metadata) error {
    // Pad LikeCount to 8 digits for correct lexicographic sort
    // paddedLikeCount := fmt.Sprintf("%08d", data.LikeCount)
    dateTimeNow := time.Now().Format("2006-01-02 15:04")

    // Build maps for multilingual fields
	if data.Vid == "" {
		return fmt.Errorf("Vid not found")
	}

    item := map[string]dynamodbtypes.AttributeValue{
        "PK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("VIDEO#%s", data.Vid)},
        "SK": &dynamodbtypes.AttributeValueMemberS{Value: "METADATA"},

        // GSI for querying by video asc/desc
        "GSI1PK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("VIDS#")},
        "GSI1SK": &dynamodbtypes.AttributeValueMemberS{
            Value: fmt.Sprintf("MOD#%s", dateTimeNow), //  Article creation
        },

		// GSI for quering by CHAN#{Channel Name}
        "GSI2PK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("CHAN#%s", data.ChannelId)},
        "GSI2SK": &dynamodbtypes.AttributeValueMemberS{
            Value: fmt.Sprintf("UPL#%s", data.UploadDate), //  Video Upload
        },

        // Main data fields
        "vid":                    &dynamodbtypes.AttributeValueMemberS{Value: data.Vid},

        "lang":                   &dynamodbtypes.AttributeValueMemberS{Value: data.Lang},

        "channel_id":            &dynamodbtypes.AttributeValueMemberS{Value: data.ChannelId},
        "video_upload_date":      &dynamodbtypes.AttributeValueMemberS{Value: data.UploadDate}, // yt publish date
        "duration":               &dynamodbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", float64(data.Duration))},
        "channel_name":             &dynamodbtypes.AttributeValueMemberS{Value: data.ChannelName},
        "category":               &dynamodbtypes.AttributeValueMemberS{Value: data.Category},
        "video_lang":             &dynamodbtypes.AttributeValueMemberS{Value: data.VideoLang},

        // Now stored as Maps
		"title":   &dynamodbtypes.AttributeValueMemberM{Value: stringMapToAttributeValueMap(data.Title)},
        "summary": &dynamodbtypes.AttributeValueMemberM{Value: stringMapToAttributeValueMap(data.Summary)},
        "answer":  &dynamodbtypes.AttributeValueMemberM{Value: stringMapToAttributeValueMap(data.Answer)},
        "path":    &dynamodbtypes.AttributeValueMemberM{Value: stringMapToAttributeValueMap(data.Path)},
		"status":  &dynamodbtypes.AttributeValueMemberM{Value: stringMapToAttributeValueMap(data.Status)},


        "article_update_datetime": &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format("2006-01-02T15:04:05")},
        "like_count":              &dynamodbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", data.LikeCount)},
        "downsub_download_cap":    &dynamodbtypes.AttributeValueMemberS{Value: data.DownSubDownloadCap},
    }

    _, err := dynamoDBClient.PutItem(context.Background(), &dynamodb.PutItemInput{
        TableName: aws.String(dynamoDBTableName),
        Item:      item,
    })
    if err != nil {
        return fmt.Errorf("failed to push to DynamoDB: %w", err)
    }

    fmt.Println("Data pushed to DynamoDB successfully.")
    return nil
}


func pushCategoryStatsToDynamoDB(data videostate.Metadata, lang string) error {
    // Pad LikeCount to 8 digits for correct lexicographic sort
    paddedLikeCount := fmt.Sprintf("%08d", data.LikeCount)
    dateTimeNow := time.Now().Format("2006-01-02 15:04")

    item := map[string]dynamodbtypes.AttributeValue{
        "PK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("LANG#%s", lang)},
        "SK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("CAT#%s#CREATED#%s", data.Category, dateTimeNow)},

        // GSI for querying by category and LikeCount
        "GSI1PK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("CAT#%s", data.Category)},
        "GSI1SK": &dynamodbtypes.AttributeValueMemberS{
            Value: fmt.Sprintf("LANG#%s#LIKES#%s", lang, paddedLikeCount),
        },

        // Main data fields
        "vid":                    &dynamodbtypes.AttributeValueMemberS{Value: data.Vid},
        "lang":                   &dynamodbtypes.AttributeValueMemberS{Value: lang},
        "uploader_name":            &dynamodbtypes.AttributeValueMemberS{Value: data.ChannelName},
        "video_upload_date":      &dynamodbtypes.AttributeValueMemberS{Value: data.UploadDate}, // yt publish date
        "category":               &dynamodbtypes.AttributeValueMemberS{Value: data.Category},
		"video_lang":             &dynamodbtypes.AttributeValueMemberS{Value: data.VideoLang},

        // Now stored as Maps
		"title":   &dynamodbtypes.AttributeValueMemberS{Value: data.Title[lang]},
        "path":    &dynamodbtypes.AttributeValueMemberS{Value: data.Path[lang]},

        "like_count":              &dynamodbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", data.LikeCount)},
    }

    _, err := dynamoDBClient.PutItem(context.Background(), &dynamodb.PutItemInput{
        TableName: aws.String(dynamoDBTableName),
        Item:      item,
    })
    if err != nil {
        return fmt.Errorf("failed to push to DynamoDB: %w", err)
    }

    fmt.Println("Data pushed to DynamoDB successfully.")
    return nil
}

func parseJSONContent(jsonString string) (map[string]string, error) {
	log.Printf("Parsing JSON: %s", jsonString)
    if strings.TrimSpace(jsonString) == "" {
        return nil, fmt.Errorf("input JSON string is empty")
    }

    var result map[string]string
    err := json.Unmarshal([]byte(jsonString), &result)
    if err != nil {
        return nil, fmt.Errorf("error parsing JSON: %v", err)
    }

    return result, nil
}


func parseFields(input string) (VideoGPTSummary, error)  {

	// Helper functions needed:
	getStringPtr := func (s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}

	getInt64Ptr := func(s string) *int64 {
		if s == "" {
			return nil
		}
		val, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil
		}
		return &val
	}

	getTimePtr := func(s string) *time.Time {
		if s == "" {
			return nil
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil
		}
		return &t
	}

    // Regex with (?s) flag to make . match newlines
    re := regexp.MustCompile(`(?s)╔\$(.*?)╗`)
    matches := re.FindAllStringSubmatch(input, -1)

	fmt.Println("matches length", len(matches))
	fmt.Println("FindAllStringSubmatch matches : ", matches)

    result := make(map[string]string)

    for _, match := range matches {
        if len(match) < 2 {
            continue
        }
        
        fieldContent := strings.TrimSpace(match[1])
        parts := strings.SplitN(fieldContent, ":", 2)
        if len(parts) != 2 {
            continue
        }
        
        fieldName := strings.TrimSpace(parts[0])
        fieldValue := strings.TrimSpace(parts[1])
        result[fieldName] = fieldValue
    }

    // jsonData, err := json.Marshal(result)
    // if err != nil {
    //     return "", fmt.Errorf("error marshaling to JSON: %v", err)
    // }

    summary := VideoGPTSummary{
		Answer:      result["answer"],
		Content:     result["content"],
		Lang:        result["lang"],
		Title:       result["title"],
		Type:        getStringPtr(result["type"]),
		Likes:       getInt64Ptr(result["likes"]),
		ChannelName: getStringPtr(result["chanel_name"]),
		ChannelId:   getStringPtr(result["channel_identifier"]),
		PublishDate: getTimePtr(result["publish_date"]),
	}

	return summary, nil
}

// Using os.ReadDir (preferred for newer Go versions)
func listPromptsDir(promptsDir string) error {
    entries, err := os.ReadDir(promptsDir)
    if err != nil {
        return fmt.Errorf("failed to read prompts directory: %w", err)
    }

    fmt.Printf("Contents of %s:\n", promptsDir)
    for _, entry := range entries {
        // Get file info
        info, err := entry.Info()
        if err != nil {
            continue
        }
        // Print file name and size
        fmt.Printf("- %s (size: %d bytes)\n", entry.Name(), info.Size())
    }
    return nil
}


// ExtractLastTimestamp extracts the last end timestamp from SRT content
func ExtractLastTimestamp(content string) (string, error) {
	re := regexp.MustCompile(`(\d{1,2}:\d{2}:\d{2}),\d{3}\s*-->\s*(\d{1,2}:\d{2}:\d{2}),\d{3}`)
	matches := re.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		return "", fmt.Errorf("no timestamp found")
	}

	lastTimestamp := matches[len(matches)-1][2]
	return lastTimestamp, nil
}

// TestCase holds input and expected output
type TestCase struct {
	name     string
	input    string
	expected string
}
func ExtractLastTimestamp_yt_dlp(content string) (string, error) {
    re := regexp.MustCompile(`\d{2}:\d{2}:\d{2}\.\d{3} --> (\d{2}:\d{2}:\d{2})\.\d{3}`)
    matches := re.FindAllStringSubmatch(content, -1)

    if len(matches) == 0 {
        return "", fmt.Errorf("nenhum timestamp encontrado")
    }

    // Pega o último timestamp de saída (apenas hh:mm:ss)
    lastTimestamp := matches[len(matches)-1][1]

    return lastTimestamp, nil
}


// Extra: Se quiser converter para time.Duration
func ParseTimestampToDuration(ts string) (int, error) {
    parsed, err := time.Parse("15:04:05", ts)
    if err != nil {
        return 0, err
    }
    
    // Calculate total seconds
    totalSeconds := parsed.Hour()*3600 + parsed.Minute()*60 + parsed.Second()
    return totalSeconds, nil
}


func summarizeText(caption string, lang string, title string) (string, error) {

	ts, errExtraction := ExtractLastTimestamp(caption)
	if errExtraction != nil {
		return "", fmt.Errorf("failed to extract last timestamp: %w", errExtraction)
	}
	tsToDuration, errDuration := ParseTimestampToDuration(ts)
	if errDuration != nil {
		return "", fmt.Errorf("failed to parse timestamp to duration: %w", errDuration)
	}

	println("ts",ts)
	println("tsToDuration: ", tsToDuration)
	println("seconds",(20*60))
	
	summary, err := llmModelSummarize(title, lang, caption )
	if (err != nil) {
		return "", fmt.Errorf("failed to summarize text: %w", err)
	}
	return string(summary.Result), nil
}

// Struct to send request
type SummarizeRequest struct {
	Model          string `json:"model"`
	PromptTemplate string `json:"prompt_template"`
	Input          struct {
		Language string `json:"language"`
		Title    string `json:"title"`
		Captions string `json:"captions"`
	} `json:"input"`
}

// Struct for response
type SummarizeResponse struct {
	Result          string `json:"result"`
	Error           string      `json:"error,omitempty"`
	RequestDuration string      `json:"request_duration"`
}

// Calls llm-model service
func llmModelSummarize(title string, lang string, caption string) (*SummarizeResponse, error) {
	url := "http://llm-model:3030/summarize" // service name from docker-compose

	// Build request payload
	payload := SummarizeRequest{
		Model:          "gemini-2.0-flash",       // deepseekr1 or "gemini-2.0-flash"
		PromptTemplate: "prompt1",          // choose which template
	}
	payload.Input.Language = lang
	payload.Input.Title = title
	payload.Input.Captions = caption

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode payload: %w", err)
	}

	// Send request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call llm-model: %w", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm-model returned status %s: %s", resp.Status, string(body))
	}

	var summarizeResp SummarizeResponse
	if err := json.Unmarshal(body, &summarizeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &summarizeResp, nil
}


func sanitizeSubtitle(subtitle string) string {
	re := regexp.MustCompile(`\s*\n\n[\s\S]*?</c>\n`)
	summarizedSrt := re.ReplaceAllString(subtitle, "\n")
	summarizedSanitized := strings.ReplaceAll(summarizedSrt, "align:start position:0%", "")
	return summarizedSanitized
}


func getSummaryFromDynamoDB(vid string, lang string) (map[string]dynamodbtypes.AttributeValue, error) {
    key := map[string]dynamodbtypes.AttributeValue{
        "PK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("VIDEO#%s", vid)},
        "SK": &dynamodbtypes.AttributeValueMemberS{Value: "METADATA"},
    }

    result, err := dynamoDBClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
        TableName: aws.String(dynamoDBTableName),
        Key:       key,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to get item from DynamoDB: %w", err)
    }

    if result.Item == nil {
        return nil, nil
    }

    // Helper to extract the correct lang value
    // extractLangValue := func(attr dynamodbtypes.AttributeValue) dynamodbtypes.AttributeValue {
    //     if m, ok := attr.(*dynamodbtypes.AttributeValueMemberM); ok {
    //         if val, exists := m.Value[lang]; exists {
    //             return val
    //         }
    //     }
    //     return &dynamodbtypes.AttributeValueMemberS{Value: ""}
    // }

    // // Replace the maps with just the single-language values
    // if val, ok := result.Item["summary"]; ok {
    //     result.Item["summary"] = extractLangValue(val)
    // }
    // if val, ok := result.Item["answer"]; ok {
    //     result.Item["answer"] = extractLangValue(val)
    // }
    // if val, ok := result.Item["path"]; ok {
    //     result.Item["path"] = extractLangValue(val)
    // }

    return result.Item, nil
}

func firstNonEmptyFromMap(m map[string]string) string {
    for _, v := range m {
        if v != "" {
            return v
        }
    }
    return "" // if all are empty
}


func getLatestVideosByCategoryFromDynamoDB(lang string, category string, minLikes int, limit int) ([]videostate.Metadata, error) {
    input := &dynamodb.QueryInput{
        TableName:              aws.String(dynamoDBTableName),
        KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
        ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
            ":pk": &dynamodbtypes.AttributeValueMemberS{
                Value: fmt.Sprintf("LANG#%s", lang),
            },
            ":sk": &dynamodbtypes.AttributeValueMemberS{
                Value: fmt.Sprintf("CAT#%s", category),
            },
        },
        ScanIndexForward: aws.Bool(false),             // newest first
        Limit:            aws.Int32(int32(limit * 2)), // fetch extra to filter manually
    }

    result, err := dynamoDBClient.Query(context.TODO(), input)
    if err != nil {
        return nil, fmt.Errorf("failed to query videos by category: %w", err)
    }

    var filtered []videostate.Metadata

    for _, item := range result.Items {
        // Extract like_count
        likeAttr, ok := item["like_count"].(*dynamodbtypes.AttributeValueMemberN)
        if !ok {
            continue
        }
        likeCount, err := strconv.Atoi(likeAttr.Value)
        if err != nil || likeCount < minLikes {
            continue
        }

        var meta videostate.Metadata

        // Extract safe values
        if v, ok := item["vid"].(*dynamodbtypes.AttributeValueMemberS); ok {
            meta.Vid = v.Value
        }
        if v, ok := item["lang"].(*dynamodbtypes.AttributeValueMemberS); ok {
            meta.Lang = v.Value
        }
        if v, ok := item["category"].(*dynamodbtypes.AttributeValueMemberS); ok {
            meta.Category = v.Value
        }
        if v, ok := item["video_upload_date"].(*dynamodbtypes.AttributeValueMemberS); ok {
            meta.UploadDate = v.Value
        }
        if v, ok := item["uploader_id"].(*dynamodbtypes.AttributeValueMemberS); ok {
            meta.ChannelId = v.Value
        }
        if v, ok := item["channel_name"].(*dynamodbtypes.AttributeValueMemberS); ok {
            meta.ChannelName = v.Value
        }

        // Handle multilingual fields (string → map)
        if v, ok := item["title"].(*dynamodbtypes.AttributeValueMemberS); ok {
            meta.Title = map[string]string{lang: v.Value}
        }
        if v, ok := item["path"].(*dynamodbtypes.AttributeValueMemberS); ok {
            meta.Path = map[string]string{lang: v.Value}
        }

        // Set like count
        meta.LikeCount = likeCount

        filtered = append(filtered, meta)
        if len(filtered) >= limit {
            break
        }
    }

    return filtered, nil
}










type HandleSummaryRequestResponse struct {
	VideoID      string  			`json:"videoId"`
	Title       map[string]string  	`json:"title"`
	Path 		map[string]string  	`json:"path"`
	Content	    map[string]string	`json:"content"`
	Answer	    map[string]string	`json:"answer"`
	Status      map[string]string  	`json:"status"`
	Lang        string  			`json:"lang"`
	ChannelId  	string  			`json:"channel_id"`
	UploadDate  string  			`json:"video_upload_date"`
	ArticleUploadDateTime string 	`json:"article_update_datetime"`
	Duration    int 				`json:"duration"`
	ChannelName string 				`json:"channel_name"`
	Category    string  			`json:"category"`
	VideoLang   string  			`json:"video_lang"`
	LikeCount 	int					`json:"like_count"`
	CanBeRetried map[string]bool	`json:"can_be_retried"`
}

type HandleSummarySingleLanguageRequestResponse struct {
	VideoID      	string  `json:"videoId"`
	Title        	string  `json:"title"`
	Path 		 	string  `json:"path"`
	Paths			map[string]string	`json:"paths"`
	Content	     	string	`json:"content"`
	Answer	     	string	`json:"answer"`
	Status       	string  `json:"status"`
	Lang        	string  `json:"lang"`
	ChannelId  		string  `json:"channel_id"`
	UploadDate  	string  `json:"video_upload_date"`
	ArticleUploadDateTime string `json:"article_update_datetime"`
	Duration    	int 	`json:"duration"`
	ChannelName   	string  `json:"channel_name"`
	Category    	string  `json:"category"`
	VideoLang   	string  `json:"video_lang"`
	LikeCount 		int		`json:"like_count"`
	CanBeRetried 	bool  	`json:"can_be_retried"`
	
}
type VideoGPTSummary struct {
    Answer           string     `json:"answer"`
    Content          string     `json:"content"`
    Lang            string     `json:"lang"`
    Title           string     `json:"title"`
    Type            *string    `json:"type,omitempty"`
    Likes           *int64     `json:"likes,omitempty"`
    ChannelName     *string    `json:"channel_name,omitempty"`
    ChannelId      *string    `json:"channel_identifier,omitempty"`
    PublishDate     *time.Time `json:"publish_date,omitempty"`
}

type DynamoDbResponseToJson struct {
	Vid        			string `dynamodbav:"vid" json:"vid"`
    Title      			map[string]string `dynamodbav:"title" json:"title"`
    Summary     		map[string]string `dynamodbav:"summary" json:"content"`
	Answer     			map[string]string `dynamodbav:"answer" json:"answer"`
	Path       			map[string]string `dynamodbav:"path" json:"path"`
    Status     			map[string]string `dynamodbav:"status" json:"status"`
	Category   			string `dynamodbav:"category" json:"category"`
	LikeCount  			int    `dynamodbav:"like_count" json:"like_count"`
    Lang       			string `dynamodbav:"lang" json:"lang"`
	VideoLang  			string `dynamodbav:"video_lang" json:"videoLang"`
    ChannelId 			string `dynamodbav:"channel_id" json:"channel_id"`
    UploadDate 			string `dynamodbav:"video_upload_date" json:"uploadDate"`
	ArticleUploadDateTime string `dynamodbav:"article_update_datetime" json:"articleUploadDateTime"`
    Duration   			int `dynamodbav:"duration" json:"duration"` // or float64 if needed
	ChannelName   		string `dynamodbav:"channel_name" json:"channel_name"`
	DownsubDownloadCap 	string `dynamodbav:"downsub_download_cap" json:"downsubDownloadCap"`
}

func isThisStatusProcessing(currentStatus string) bool {
	if strings.HasPrefix(currentStatus, "processing-") || strings.HasPrefix(currentStatus, "download-") || strings.HasPrefix(currentStatus, "metadata-"){
		return true
	}
	return false	
}

func isMoreThanThreeMinutesOld(uploadDateTime string) (bool, error) {
    // Tenta como timestamp Unix
    if uploadTime, err := strconv.ParseInt(uploadDateTime, 10, 64); err == nil {
        articleTime := time.Unix(uploadTime, 0)
        return time.Since(articleTime) > 3*time.Minute, nil
    }

    // Tenta como RFC3339
    if articleTime, err := time.Parse(time.RFC3339, uploadDateTime); err == nil {
        return time.Since(articleTime) > 3*time.Minute, nil
    }

    // Tenta como formato sem timezone
    if articleTime, err := time.Parse("2006-01-02T15:04:05", uploadDateTime); err == nil {
        return time.Since(articleTime) > 3*time.Minute, nil
    }

    return false, fmt.Errorf("failed to parse datetime: %s", uploadDateTime)
}



func loadContentWhenItsCached(videoID string, lang string) (videostate.Metadata, error) {
	metadata := videostate.Metadata{}
	type ContentData struct {
		Content string `json:"content"`
		Answer  string `json:"answer"`
	}

	cachedData, err := getSummaryFromDynamoDB(videoID, lang)
	if err != nil {
		log.Printf("❌ Error fetching from DynamoDB: %v", err)
		return metadata, err
	}

	var dynamoDbResponse DynamoDbResponseToJson
	if err := attributevalue.UnmarshalMap(cachedData, &dynamoDbResponse); err != nil {
		log.Printf("❌ Failed to unmarshal DynamoDB item: %v", err)
		return metadata, err
	}

	log.Printf("✅ Loaded DynamoDB content: status=%s, title=%s", dynamoDbResponse.Status, dynamoDbResponse.Title)

	
	// Check if it's processing in DynamoDb
	if dynamoDbResponse.Status != nil{
		
		return videostate.Metadata{
			Title:                 dynamoDbResponse.Title,
			Vid:                   videoID,
			Summary:               dynamoDbResponse.Summary,
			Category:              dynamoDbResponse.Category,
			Lang:                  lang,
			Answer:                dynamoDbResponse.Answer,
			Path:                  dynamoDbResponse.Path,
			Status:                dynamoDbResponse.Status,
			ChannelId:             dynamoDbResponse.ChannelId,
			UploadDate:            dynamoDbResponse.UploadDate,
			ArticleUploadDateTime: dynamoDbResponse.ArticleUploadDateTime,
			Duration:              dynamoDbResponse.Duration,
			LikeCount: 			   dynamoDbResponse.LikeCount,
			ChannelName: 			   dynamoDbResponse.ChannelName,
			DownSubDownloadCap:    dynamoDbResponse.DownsubDownloadCap,
			VideoLang: 		       dynamoDbResponse.VideoLang,
		}, nil
	}

	log.Println("ℹ️ No valid cached data found.")
	return metadata, nil
}

type MetadataParams struct {
    VideoID                   string
    Language                  string
    MetadataDynamoResponse   *HandleSummaryRequestResponse
    FetchMetadataResponse    *VideoMetadata
}
func processingQueueVideoGetMetadata(params MetadataParams) (
	*HandleSummaryRequestResponse,
	*VideoMetadata,
	error,
) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", params.VideoID)
	var videoProcessingMetadataDTO = videostate.ProcessingVideo{
		VideoID:  params.VideoID,
		Language: params.Language,
	}

	ttlMetadata := videoQueue.GetTTLMetadata(params.VideoID, params.Language)
	println("GET METADATA TTL", ttlMetadata)
	if ttlMetadata < 1 {
		videoQueue.SetStatus(params.VideoID, params.Language, videostate.StatusMetadataTTlExceeded)

		go func() {
			var metadata = videoQueue.GetVideoMeta(params.VideoID, params.Language)
			metadata.Vid = params.VideoID
			metadata.Lang = params.Language
			if err := pushMetadataToDynamoDB(*metadata); err != nil {
				log.Printf("❌ Failed to push metadata to DynamoDB: %v", err)
			} else {
				log.Printf("✅ Pushed expired metadata from %s to DynamoDB", params.VideoID)
			}
		}()
		return nil, nil, fmt.Errorf("ttlMetadata < 1")
	}

	// Fetch Metadata
	log.Println("⏳ => Fetching Metadata", params.VideoID)
	metadataDynamoResponse, fetchMetadataResponse, err :=
		runMetadataAndCapsFetcherAsync(videoURL, params.Language)
	log.Println("after runMetadataAndCapsFetcherAsync")

	if err != nil {
		log.Printf("❌ Failed to run metadata fetch after retry: %v", err)
		time.Sleep(1 * time.Second)
		return nil, nil, err
	}

	if fetchMetadataResponse.Category == "" {
		log.Printf("❌ [0] No category found for video %s", params.VideoID)
		time.Sleep(1 * time.Second)
		return nil, nil, fmt.Errorf("no category found")
	}

	log.Println("before convertHandleSummary")
	videoMetadata := convertHandleSummaryRequestResponseToVideoStateMetadata(metadataDynamoResponse)
	log.Println("after convertHandleSummary")

	videoProcessingMetadataDTO.Metadata = videoMetadata

	if len(fetchMetadataResponse.Captions) == 0 {
		log.Printf("❌ No captions found for video %s", params.VideoID)
		time.Sleep(1 * time.Second)
		return nil, nil, fmt.Errorf("no captions found")
	}

	downSubUrl := fetchMetadataResponse.Captions[0].BaseURL
	videoProcessingMetadataDTO.Metadata.DownSubDownloadCap = downSubUrl

	path := convertTitleToURL(videoMetadata.Title[params.Language])
	if videoProcessingMetadataDTO.Metadata.Path == nil {
		videoProcessingMetadataDTO.Metadata.Path = make(map[string]string)
	}
	videoProcessingMetadataDTO.Metadata.Path[params.Language] = path

	log.Println("🔄 Update metadata on videoProcessing:", params.VideoID)
	videoQueue.Add(videoProcessingMetadataDTO)

	log.Println("[1] Set Status", videostate.StatusMetadataProcessed)
	videoQueue.SetStatus(params.VideoID, params.Language, videostate.StatusMetadataProcessed)

	go func() {
		metadata := videoQueue.GetVideoMeta(params.VideoID, params.Language)
		if metadata.Category == "" {
			log.Printf("❌ [2] No category found for video %s", params.VideoID)
			time.Sleep(1 * time.Second)
		} else {
			println("🔄 [2] printing category:", videoMetadata.Category)
		}

		if err := pushMetadataToDynamoDB(*metadata); err != nil {
			log.Printf("❌ Failed to push metadata to DynamoDB: %v", err)
		}
	}()

	return metadataDynamoResponse, fetchMetadataResponse, nil
}

func processingQueueVideoDownloadCaps(videoId string,language string, videoProcessingMetadataDTO videostate.ProcessingVideo ) videostate.ProcessingVideo{
	
	metadata := videoQueue.GetVideoMeta(videoId, language)
			
	// Download Caps
	log.Println("⏳ => Download Caps", videoId)
	downloadUrl := metadata.DownSubDownloadCap
	subtitle, err := downloadSubtitleByDownSub(downloadUrl)
	if (err != nil) {
		log.Printf("❌ Failed to download subtitle: %v", err)
		time.Sleep(1 * time.Second)
		return videoProcessingMetadataDTO
	}
	
	videoProcessingMetadataDTO = videostate.ProcessingVideo{
		VideoID: videoId,
		Language: language,
		SubtitleContent: subtitle,
	}
	
	videoQueue.Add(videoProcessingMetadataDTO)
	log.Println("🔄 Update metadata on to include DownSubDownloadCap")

	log.Println("[2] Set Status ", videostate.StatusDownloadProcessed)
	videoQueue.SetStatus(videoId, language, videostate.StatusDownloadProcessed)

	go func(){
		subtitleKey := videoId + "-caption.txt"
		if err := uploadToS3(subtitle, subtitleKey); err != nil {
			log.Printf("Error uploading subtitle to S3: %v\n", err)
		}
	}()

	return videoProcessingMetadataDTO
}

func processingQueueVideoSummarize(videoId string, language string, videoProcessingMetadataDTO videostate.ProcessingVideo) videostate.ProcessingVideo{
	log.Println("⏳ => Summarize it", videoId)
	var metadata = videoQueue.GetVideoMeta(videoId, language)
	var subtitle = videoProcessingMetadataDTO.SubtitleContent
	if (subtitle == "") {
		log.Println("NO SUbtitle found")
		subtitleKey := videoId + "-caption.txt"
		subtitle, _ = fetchS3(subtitleKey)
	}

	// set the same title for other languages
	// here is where we can translate the title so the path would be translated as well.
	title := firstNonEmptyFromMap(metadata.Title)
	title = sanitizeTitle(title)
	metadata.Title[language] = title

	var path = convertTitleToURL(metadata.Title[language])
	if metadata.Path == nil {
		metadata.Path = make(map[string]string)
	}
	metadata.Path[language] = path

	prompt, err := summarizeText(subtitle, language, metadata.Title[language])
	println("summarizeText prompt; ", prompt)
	if err != nil {
		log.Printf("error summarizing caption: %v", err)
		return videoProcessingMetadataDTO
	}
	
	summaryJson, err := summarizeSubtitle(prompt)
	if (err != nil) {
		log.Printf("❌ Failed to summarize subtitle: %v", err)
		time.Sleep(1 * time.Second)
		return videoProcessingMetadataDTO
	}
	if metadata.Answer== nil {
		metadata.Answer = make(map[string]string)
	}
	metadata.Answer[language] = summaryJson.Answer

	if metadata.Summary == nil {
		metadata.Summary = make (map[string]string)
	}
	metadata.Summary[language] = summaryJson.Content

	videoProcessingMetadataDTO.Metadata = *metadata
	videoQueue.Add(videoProcessingMetadataDTO)

	log.Println("🚀 [2] Set Status ", videostate.StatusSummarizeProcessed)
	videoQueue.SetStatus(videoId, language, videostate.StatusSummarizeProcessed)
	
	// videoQueue.SetRetrySummaryStatus(videoId, language, false) // this prevent looping when it's on retrying

	go func(){
		var metadata = videoQueue.GetVideoMeta(videoId, language)
		if err := pushMetadataToDynamoDB(*metadata); err != nil {
			log.Printf("❌ Failed to push metadata to DynamoDB: %v", err)
		}else{
			if err := pushCategoryStatsToDynamoDB(*metadata, language); err != nil{
				log.Printf("❌ Failed to push category to DynamoDB: %v", err)
			}
		}
	}()

	return videoProcessingMetadataDTO
}


func processingVideoQueue(videoId string, language string) {
	println("processingVideoQueue")
	ttl := 3
	//videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoId)
	var metadataDynamoResponse *HandleSummaryRequestResponse
	var fetchMetadataResponse *VideoMetadata
	var videoProcessingMetadataDTO = videostate.ProcessingVideo{
		VideoID: videoId,
		Language: language,
	}

	// direct-video-digest or download-and-digest
	var summaryType = videoQueue.GetPipeline(videoId, language)

	videoQueue.SetTTLMetadata(videoId, language, ttl)
	
	for videoQueue.Exists(videoId, language){
		if (videoQueue.GetStatus(videoId, language) == videostate.StatusSummarizeProcessed) {
			println("COMPLETED")
			break
		}

		//Error Handler
		if strings.Contains(strings.ToLower(string(videoQueue.GetStatus(videoId, language))), "error") {
			var metadata = videoQueue.GetVideoMeta(videoId, language)
			// metadata.ArticleUploadDateTime is string. Convert it to time Time
			
			isOld, err := isMoreThanThreeMinutesOld(metadata.ArticleUploadDateTime)
			if err != nil {
			    // Handle error
			    log.Printf("Error checking article time: %v", err)
			}
			
			if !isOld {
				log.Printf("Video %s should wait for 3 minutes before resuming.", videoId)
				break
			}

			println("ArticleUploadDateTime : ", metadata.ArticleUploadDateTime)
			videoQueue.SetStatus(videoId, language, videostate.StatusPending)
			continue;
		}
		println("[1] Video Status : ", videoQueue.GetStatus(videoId, language))
		

		// direct-video-digest or download-and-digest
		if (summaryType == ""){

		}
		
		// Metadata Handler
		println(">>>>>>>>>>> BEFORE Fetch metadata")
		if (videoQueue.GetStatus(videoId, language) == videostate.StatusPending){
			videoQueue.DecreaseTTLMetadata(videoId, language)
			params := MetadataParams{
				VideoID:                videoId,
				Language:               language,
				MetadataDynamoResponse: metadataDynamoResponse,
				FetchMetadataResponse:  fetchMetadataResponse,
			}
			var err error
			metadataDynamoResponse, fetchMetadataResponse, err = processingQueueVideoGetMetadata(params)

			if err != nil {
				break;
			}
		}
		// Check the status
		println("[1] GET Status: ", videoQueue.GetStatus(videoId, language))

		// Download Handler
		println(">>>>>>>> BEFORE Download")
		if (videoQueue.GetStatus(videoId, language) == videostate.StatusMetadataProcessed){
			videoProcessingMetadataDTO = processingQueueVideoDownloadCaps(videoId, language, videoProcessingMetadataDTO)
		}
		
		// Summarize Handle r
		println(">>>>>>>> BEFORE Summarize")
		if (videoQueue.GetStatus(videoId, language) == videostate.StatusDownloadProcessed){
			processingQueueVideoSummarize(videoId, language, videoProcessingMetadataDTO )
			break
		}
		// Check the status
		println("Final GET Status: ", videoQueue.GetStatus(videoId, language))
		time.Sleep(1 * time.Second)
		videoQueue.DecreaseTTLMetadata(videoId, language)
		ttl--
	}
}

func dump_print_all_videos(){
	println("⌛ [1] Printing all videos from videoQueue.Videos()")
	for _, video := range videoQueue.Videos() {
		println("[1] ===> VideoID: ", video.VideoID, " Language: ", video.Language, " Status: ", video.Status)
	}
}

func convertMultilingualToSingleLingual(multilingual *HandleSummaryRequestResponse, language string) *HandleSummarySingleLanguageRequestResponse {
	urls := make(map[string]string)
	sumtubeBaseUrl := os.Getenv("BASE_URL")
	// Loop through the map and convert each value
	for key, _ := range multilingual.Path {
		// Convert the value to a string and append it to the new map
		urls[key] = fmt.Sprintf("%s/%s/%s", sumtubeBaseUrl, key, multilingual.VideoID)
	}
	
	return &HandleSummarySingleLanguageRequestResponse{
		VideoID:		    multilingual.VideoID,
		Path: 				multilingual.Path[language],
		Paths:				urls,
		Title: 				multilingual.Title[language],
		Answer: 			multilingual.Answer[language],
		Content: 			multilingual.Content[language],
		Status: 			multilingual.Status[language],			
		Lang:               multilingual.VideoLang,
		ChannelId:          multilingual.ChannelId,
		UploadDate:         multilingual.UploadDate,   
		ArticleUploadDateTime: multilingual.ArticleUploadDateTime,
		Duration:           multilingual.Duration,
		ChannelName:        multilingual.ChannelName,
		Category:           multilingual.Category,
		VideoLang:          multilingual.VideoLang,
		LikeCount:          multilingual.LikeCount,
		CanBeRetried:		multilingual.CanBeRetried[language],
	}
}

func handleSummaryRequest(w http.ResponseWriter, r *http.Request) {

    if r.Method != http.MethodPost {
        http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
        return
    }
	// check if exist retry param
	// if exist, retry the video
	var retrySummaryUrlQuery bool 
	retryParam := strings.ToLower(r.URL.Query().Get("retry"))
	retrySummaryUrlQuery = retryParam == "true" || retryParam == "1" || retryParam == "yes"

	println("retrySummaryUrlQuery:", retrySummaryUrlQuery)


	// get prompt http://localhost:8080/summary/{type} it could be direct-video-digest or download-and-digest
	parts := strings.Split(r.URL.Path, "/")
	fragmentType := ""
	if len(parts) < 3 {
		fragmentType = "download-and-digest"
		//http.Error(w, "missing type", http.StatusBadRequest)
		// return
	}else {
		fragmentType = parts[2]
	}

	// print all videos from videoQueue.Videos()
	dump_print_all_videos()
	println("dump_print_all_videos 1")
	
    var requestBody struct {
        VideoID  string `json:"videoId"`
        Language string `json:"language"`
    }
    if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

	lang := requestBody.Language
    videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", requestBody.VideoID)
 
	videoID, err := extractVideoID(videoURL)
    if err != nil {
        http.Error(w, fmt.Sprintf("Error extracting video ID: %v", err), http.StatusBadRequest)
        return
    }

	videoQueue.SetPipeline(videoID, lang, fragmentType)
	
	isVideoBeingProcessed := videoQueue.Exists(videoID, lang)

	println("isVideoProcessing", isVideoBeingProcessed)
	println("retrySummaryUrlQuery", retrySummaryUrlQuery)

	if ( !isVideoBeingProcessed ) {
		//dump_print_all_videos()
		videoProcessingMetadataDTO := videostate.ProcessingVideo{
			VideoID: videoID,
			Language: lang,
		}
		println("Add video to Queue")
		videoQueue.Add(videoProcessingMetadataDTO)
		println("Set Status", videostate.StatusPending)
		videoQueue.SetStatus(videoID, lang, videostate.StatusPending)
		println("Processing video async 1")
		content, _ := loadContentWhenItsCached(videoID, lang)
		println("loading metadata")
		metadata := videostate.Metadata{}
		println("content.Vid and status = ",content.Vid, content.Status, content.Path)

		// check if answer or summary does not exist in DynamoDb for this language
		if (content.Answer[lang] == "" && content.Summary[lang] == "" ) {
			// deleteVideoFromDynamoDB(videoID)
			println("❌ Answer or Summary missing in DynamoDB for language:", lang)
			content.Vid = ""
		}

		if content.Vid  != "" {
			println("🧠 Parsing cached summary content")
			status := content.Status

			if (status[lang] == "" && len(status) > 0 ) {
				status[lang] = string(videostate.StatusDownloadProcessed)
			}
			//Convert DynamoDb fields to Metadata fields
			metadata = videostate.Metadata{
				Title:                 content.Title,
				Vid:                   content.Vid,
				Status:                status,
				Summary:               content.Summary,
				Answer:                content.Answer,
				Category:              content.Category,
				Lang:                  content.Lang,
				VideoLang: 	   		   content.VideoLang,
				Path:                  content.Path,
				ChannelId:             content.ChannelId,
				UploadDate:            content.UploadDate,
				ChannelName: 		   content.ChannelName,
				ArticleUploadDateTime: content.ArticleUploadDateTime,
				Duration:              content.Duration, 
				LikeCount: 			   content.LikeCount,
				DownSubDownloadCap:    content.DownSubDownloadCap,
			}

			videoProcessingMetadataDTO.Metadata = metadata
			
			println("Add video to Queue")
			videoQueue.Add(videoProcessingMetadataDTO)

			println("Set Status", videostate.VideoStatus(metadata.Status[lang]))
			videoQueue.SetStatus(videoID, lang, videostate.VideoStatus(metadata.Status[lang]))

			canBeRetried := videoQueue.CanBeRetried(videoID, lang)
			if retrySummaryUrlQuery  && canBeRetried  {
				videoQueue.SetRetrySummaryStatus(videoID, lang, retrySummaryUrlQuery)
				videoQueue.SetStatus(videoID, lang, videostate.StatusPending)
			}
			
			println("Processing video sync", videoID, lang)
			// Processing video sync
			go func(){
				processingVideoQueue(videoID, lang)
			}()
			
		} else {
			println("Processing video async 2")
			go func(){
				// Processing video async 
				videoQueue.SetStatus(videoID, lang, videostate.StatusPending)
				processingVideoQueue(videoID, lang)
			}()
		}
	}

	canBeRetried := videoQueue.CanBeRetried(videoID, lang)
	multilingualCanBeRetried := map[string]bool{
		lang: canBeRetried,
	}
	println("canBeRetried", canBeRetried)
	// Handle retry requests
	if ( retrySummaryUrlQuery && canBeRetried ) {
		println("PROCESS ON RETRY")
		videoQueue.SetStatus(videoID, lang, videostate.StatusPending)
		videoQueue.SetRetrySummaryStatus(videoID, lang, true)
	}
	
	currentMetadata := videoQueue.GetVideoMeta(videoID, lang)
	
	// Return here
	response := HandleSummaryRequestResponse{
		VideoID:     videoID,
		Title:       currentMetadata.Title,
		Lang:        currentMetadata.Lang,
		VideoLang:   currentMetadata.VideoLang,
		Path:		 currentMetadata.Path,
		Status:      currentMetadata.Status,
		ChannelId:   currentMetadata.ChannelId,
		UploadDate:  currentMetadata.UploadDate,
		ChannelName:   currentMetadata.ChannelName,
		ArticleUploadDateTime: currentMetadata.ArticleUploadDateTime,
		Duration:    currentMetadata.Duration,
		Category:    currentMetadata.Category,
		LikeCount:   currentMetadata.LikeCount,
		Content:	 currentMetadata.Summary,
		Answer:		 currentMetadata.Answer,
		CanBeRetried: multilingualCanBeRetried,
	}

	singleLangResponse := convertMultilingualToSingleLingual(&response, lang)

	log.Printf("Processing videoID=%s,", videoID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(singleLangResponse)
	return
    
}

func blockingRetry(
	attempts int,
	timeout time.Duration,
	delay time.Duration,
	fn func() error,
) error {
	var lastErr error

	for i := 0; i < attempts; i++ {
		errChan := make(chan error, 1)

		go func() {
			errChan <- fn()
		}()

		select {
		case err := <-errChan:
			if err == nil {
				return nil
			}
			log.Printf("⚠️ Attempt %d failed: %v", i+1, err)
			lastErr = err
		case <-time.After(timeout):
			log.Printf("⏱️ Attempt %d timed out after %s", i+1, timeout)
			lastErr = fmt.Errorf("timeout after %s", timeout)
		}

		time.Sleep(delay)
	}

	return fmt.Errorf("all %d attempts failed: %v", attempts, lastErr)
}


func runWithTimeout(timeout time.Duration, fn func() error) error {
	ch := make(chan error, 1)
	go func() {
		ch <- fn()
	}()
	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("function timed out after %s", timeout)
	}
}

func convertHandleSummaryRequestResponseToVideoStateMetadata(metadata *HandleSummaryRequestResponse) videostate.Metadata {
	// check if content, anser path and status is nil. if it is initialize map
	
	if metadata.Title == nil {
		metadata.Title = make(map[string]string)
	}

	if metadata.Status == nil{
		metadata.Status = make(map[string]string)
	}
	
	if metadata.Answer == nil{
		metadata.Answer = make(map[string]string)
	}
	if metadata.Content == nil{
		metadata.Content = make(map[string]string)
	}
	if metadata.Path == nil{
		metadata.Path = make(map[string]string)
	}
	return videostate.Metadata{
		Title:                 metadata.Title,
		Vid:                   metadata.VideoID,
		Status:                metadata.Status,
		Summary:               metadata.Content,
		Answer:                metadata.Answer,
		Path:                  metadata.Path,
		Category:              metadata.Category,
		Lang:                  metadata.Lang,
		ChannelId:             metadata.ChannelId,
		UploadDate:            metadata.UploadDate,
		ChannelName: 		   metadata.ChannelName,
		ArticleUploadDateTime: metadata.ArticleUploadDateTime,
		Duration:              metadata.Duration,
		LikeCount: 			   metadata.LikeCount,
		VideoLang: 			   metadata.VideoLang,
	}
}

func runMetadataAndCapsFetcherAsync(videoURL string, lang string) (*HandleSummaryRequestResponse, *VideoMetadata, error) {
	videoID, err := extractVideoID(videoURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract video ID: %v", err)
	}

	// Get basic metadata
	metadata, err := getVideoMetadata(videoURL)
	println("RUN getVideoMetadata")
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching metadata: %v", err)
	}

	status := make(map[string]string)
	status[lang] = "processing"
	captionLang := ""
	if len(metadata.Captions) > 0 {
		captionLang = metadata.Captions[0].Lang
	} else {
		status[lang] = "caps_not_found"
	}

	durationInt, err := strconv.Atoi(metadata.LengthSeconds)
	if err != nil {
		log.Printf("⚠️ Failed to convert duration to int: %v", err)
		durationInt = 0
	}


	likeCountInt, err := strconv.Atoi(metadata.ViewCount)
	if err != nil {
		log.Printf("⚠️ Failed to convert like count to int: %v", err)
		likeCountInt = 0
	}

	var title map[string]string 
	if title == nil {
		title =  make(map[string]string)
	}
	title[lang] = metadata.Title
	// Build response object
	metadataResponse := &HandleSummaryRequestResponse{
		VideoID:     videoID,
		Title:       title,
		Lang:        lang,
		Status:      status,
		ChannelId:   metadata.ChannelId,
		UploadDate:  metadata.PublishDate,
		Duration:    durationInt,
		ChannelName:   metadata.ChannelName,
		Category:    metadata.Category,
		VideoLang:   captionLang,
		LikeCount:   likeCountInt,
	}

	// path := convertTitleToURL(metadata.Title)
	// if err := pushSummaryToDynamoDB(*metadataResponse, "", path); err != nil {
	// 	log.Printf("❌ Failed to push metadata to DynamoDB: %v", err)
	// 	return nil, nil, err
	// }

	return metadataResponse, metadata, nil
}

func summarizeSubtitle(prompt string) (*VideoGPTSummary, error) {
	sanitizedSummary, err := parseFields(prompt)
	if err != nil {
		return nil, fmt.Errorf("error sanitizing summary JSON: %v", err)
	}

	return &sanitizedSummary, nil
}


func handleCategorySummaryRequest(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	category := r.URL.Query().Get("category")
	lang := r.URL.Query().Get("lang")
	minLikesStr := r.URL.Query().Get("min_likes")
	limitStr := r.URL.Query().Get("limit")

	if category == "" || lang == "" {
		http.Error(w, "Missing 'category' or 'lang' query parameters", http.StatusBadRequest)
		return
	}

	// Defaults
	minLikes := 0
	limit := 10

	if minLikesStr != "" {
		if val, err := strconv.Atoi(minLikesStr); err == nil {
			minLikes = val
		}
	}
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil {
			limit = val
		}
	}

	// Query DynamoDB
	items, err := getLatestVideosByCategoryFromDynamoDB(lang, category, minLikes, limit)
	if err != nil {
		log.Printf("Error fetching videos by category: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	// loop thought items
	// convert items to HandleSummaryRequestResponse
	
	// Initialize the slice with the required length
	var output = make([]HandleSummarySingleLanguageRequestResponse, len(items))
	for i, item := range items {
		output[i] = convertMetadataToHandleSummaryRequestResponse(item, lang)
	}

	// Respond with JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(output)
}

// Convert videostate.Metadata to HandleSummaryRequestResponse
func convertMetadataToHandleSummaryRequestResponse(metadata videostate.Metadata, lang string) HandleSummarySingleLanguageRequestResponse {
	// Convert metadata fields to HandleSummaryRequestResponse
	return HandleSummarySingleLanguageRequestResponse{
		VideoID:     metadata.Vid,
		Title:       metadata.Title[lang],
		Lang:        metadata.Lang,
		VideoLang:   metadata.VideoLang,
		Path:        metadata.Path[lang],
		Status:      metadata.Status[lang],
		ChannelId:   metadata.ChannelId,
		UploadDate:  metadata.UploadDate,
		ChannelName:   metadata.ChannelName,
		ArticleUploadDateTime: metadata.ArticleUploadDateTime,
		Duration:    metadata.Duration,
		Category:    metadata.Category,
		LikeCount:   metadata.LikeCount,
		Content:     metadata.Summary[lang],
		Answer:      metadata.Answer[lang],
	}
}
    


// type ApiResponseSummary struct {
//     Title   string `json:"title,omitempty"`
//     Vid     string `json:"videoId"`
//     Content string `json:"content,omitempty"`
//     Lang    string `json:"lang"`
// 	Category string `json:"category,omitempty"`
//     Answer  string `json:"answer,omitempty"`
//     Path    string `json:"path,omitempty"`
//     Status  string `json:"status"` // Mandatory field
// 	ChannelId string `json:"uploader_id,omitempty"`
// 	UploadDate string `json:"video_upload_date,omitempty"`
// 	ArticleUploadDateTime string `json:"article_update_datetime,omitempty"`
// 	Duration string `json:"duration,omitempty"`
// }
// Handle Google OAuth2 redirect
func handleGoogleRedirect(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Authorization code not found", http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for an access token
	token, err := googleOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to exchange token: %v", err), http.StatusInternalServerError)
		return
	}

	// Save the access token to a file
	err = ioutil.WriteFile(".access-token", []byte(token.AccessToken), 0600)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save access token: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Successfully authenticated and saved access token to .access-token")
}

// Start Google OAuth2 flow
func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	url := googleOAuthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Set CORS headers
        w.Header().Set("Access-Control-Allow-Origin", "*") // Adjust this to be more restrictive if needed
        w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        w.Header().Set("Access-Control-Allow-Credentials", "true")
        w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

        // If this is a preflight request, respond with just the headers
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        // Call the next handler
        next(w, r)
    }
}

func main() {
	  // Create a new CORS handler
	  c := cors.New(cors.Options{
        AllowedOrigins:   []string{"*"}, // Adjust this to your frontend URL(s)
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Content-Type", "Authorization"},
        AllowCredentials: true,
        Debug:           true, // Set to false in production
    })

    // Create your router
    mux := http.NewServeMux()
	mux.HandleFunc("/summary/", handleSummaryRequest)
    mux.HandleFunc("/summary", handleSummaryRequest)
    mux.HandleFunc("/redirects", handleGoogleRedirect)
	mux.HandleFunc("/summary/category", handleCategorySummaryRequest) // New endpoint
    mux.HandleFunc("/login", handleGoogleLogin)

    // Wrap your router with the CORS handler
    handler := c.Handler(mux)

    fmt.Println("Server started at :8080 test")
    log.Fatal(http.ListenAndServe(":8080", handler))
}