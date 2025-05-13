package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"context"

	_ "embed"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
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
const dynamoDBTableName = "SummarizedSubtitles"

var s3Client *s3.Client
var dynamoDBClient *dynamodb.Client

func init() {
	fmt.Println("Initializing AWS SDK...")
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
	re := regexp.MustCompile(`(?:v=|\/)([0-9A-Za-z_-]{11}).*`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("no video ID found in URL: %s", url)
	}
	return matches[1], nil
}

type VideoMetadata struct {
	Title       string   `json:"title"`
	Language    string   `json:"language"`
	UploaderID  string   `json:"uploader_id"`
	UploadDate  string   `json:"upload_date"`
	Duration    float64  `json:"duration"`
	ChannelID   string   `json:"channel_id"`
	Categories  []string `json:"categories"`
	LikeCount   int      `json:"like_count"`
}

func getVideoMetadata(videoURL string) (string, string, string, string, float64, string, string, int, error) {
	cmd := exec.Command("yt-dlp", "--dump-json", videoURL)
	output, err := cmd.Output()
	if err != nil {
		return "", "", "", "", 0, "", "", 0, fmt.Errorf("failed to run yt-dlp: %w", err)
	}

	var metadata VideoMetadata
	if err := json.Unmarshal(output, &metadata); err != nil {
		return "", "", "", "", 0, "", "", 0, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Get first category or empty string if none
	category := ""
	if len(metadata.Categories) > 0 {
		category = metadata.Categories[0]
	}

	return metadata.Title,
		metadata.Language,
		metadata.UploaderID,
		metadata.UploadDate,
		metadata.Duration,
		metadata.ChannelID,
		category,
		metadata.LikeCount,
		nil
}

func convertTitleToURL(title string) string {
	lowercaseTitle := strings.ToLower(title)
	urlFriendly := strings.ReplaceAll(lowercaseTitle, " ", "-")
	var result strings.Builder
	for _, char := range urlFriendly {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func downloadSubtitle(videoURL string, lang string) (string, error) {
	videoID, err := extractVideoID(videoURL)
	if err != nil {
		return "", fmt.Errorf("failed to extract video ID: %w", err)
	}
	outputTemplate := filepath.Join(tempDir, "%(id)s.%(ext)s")

	// First try with the requested language
	fmt.Printf("Attempting to download subtitles in language '%s'...\n", lang)
	subtitle, err := tryDownloadSubtitle(videoURL, videoID, outputTemplate, lang)
	if err == nil {
		return subtitle, nil
	}

	// If the first attempt failed and the requested language wasn't English, try with English
	if lang != "en" {
		fmt.Printf("Failed to download subtitles in language '%s', trying English instead...\n", lang)
		subtitle, err = tryDownloadSubtitle(videoURL, videoID, outputTemplate, "en")
		if err == nil {
			return subtitle, nil
		}
	}

	// If both attempts failed, return the original error
	return "", fmt.Errorf("failed to download subtitles in either '%s' or 'en': %w", lang, err)
}

func tryDownloadSubtitle(videoURL, videoID, outputTemplate, lang string) (string, error) {
	cmd := exec.Command("yt-dlp",
		"--skip-download",
		"--write-auto-sub",
		"--sub-lang", lang,
		"--convert-subs", "srt",
		"-k",
		"-o", outputTemplate,
		videoURL,
	)
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

func pushSummaryToDynamoDB(data HandleSummaryRequestResponse, summary string, path string) error {
    // Pad LikeCount to 8 digits for correct lexicographic sort
    paddedLikeCount := fmt.Sprintf("%08d", data.LikeCount)

    item := map[string]dynamodbtypes.AttributeValue{
        "PK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("VIDEO#%s#LANG#%s", data.Vid, data.Lang)},
        "SK": &dynamodbtypes.AttributeValueMemberS{Value: "METADATA"},

        // GSI for querying by category and LikeCount
        "GSI1PK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("CATEGORY#%s#LANG#%s", data.Category,data.Lang)},
        "GSI1SK": &dynamodbtypes.AttributeValueMemberS{
            Value: fmt.Sprintf("LIKECOUNT#%s#%s#%s", paddedLikeCount, data.Vid, data.Lang),
        },

        // Main data fields
        "vid":          &dynamodbtypes.AttributeValueMemberS{Value: data.Vid},
        "title":        &dynamodbtypes.AttributeValueMemberS{Value: data.Title},
        "lang":         &dynamodbtypes.AttributeValueMemberS{Value: data.Lang},
        "status":       &dynamodbtypes.AttributeValueMemberS{Value: data.Status},
        "uploader_id":  &dynamodbtypes.AttributeValueMemberS{Value: data.UploaderID},
        "upload_date":  &dynamodbtypes.AttributeValueMemberS{Value: data.UploadDate},
        "duration":     &dynamodbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%f", data.Duration)},
        "channel_id":   &dynamodbtypes.AttributeValueMemberS{Value: data.ChannelID},
        "category":     &dynamodbtypes.AttributeValueMemberS{Value: data.Category},
        "video_lang":   &dynamodbtypes.AttributeValueMemberS{Value: data.VideoLang},
        "summary":      &dynamodbtypes.AttributeValueMemberS{Value: summary},
        "path":         &dynamodbtypes.AttributeValueMemberS{Value: path},
        "LikeCount":    &dynamodbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", data.LikeCount)},
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



func pushCategoryStatsToDynamoDB(categoryName string, likes int, vid string, date string) error {
    compositeKey := fmt.Sprintf("%s#%d#%s", categoryName, likes, date)

    item := map[string]dynamodbtypes.AttributeValue{
        "id": &dynamodbtypes.AttributeValueMemberS{Value: compositeKey},
        "data": &dynamodbtypes.AttributeValueMemberM{
            Value: map[string]dynamodbtypes.AttributeValue{
                "category": &dynamodbtypes.AttributeValueMemberS{Value: categoryName},
                "likes":    &dynamodbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", likes)},
                "vid":      &dynamodbtypes.AttributeValueMemberS{Value: vid},
                "date":     &dynamodbtypes.AttributeValueMemberS{Value: date},
            },
        },
    }

    _, err := dynamoDBClient.PutItem(context.Background(), &dynamodb.PutItemInput{
        TableName: aws.String(dynamoDBTableName),
        Item:      item,
    })
    if err != nil {
        return fmt.Errorf("failed to push category stats to DynamoDB: %w", err)
    }

    fmt.Println("Category stats pushed to DynamoDB successfully.")
    return nil
}


func parseFields(input string) (string, error) {
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

    jsonData, err := json.Marshal(result)
    if err != nil {
        return "", fmt.Errorf("error marshaling to JSON: %v", err)
    }

    return string(jsonData), nil
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


// ExtractLastTimestamp recebe o conteúdo da legenda e retorna o último timestamp (como string ou time.Duration)
func ExtractLastTimestamp(content string) (string, error) {
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
	systemPrompt := ""
	if tsToDuration < (20*60) {
		systemPrompt = system1PromptTxt
		println("using prompt: prompt1")
	}else{
		systemPrompt = system2PromptTxt
		println("using prompt: prompt2")
	}

    

    userPromptTemplate := user1PromptTxt

    // Rest of your function remains the same...
    userPrompt := fmt.Sprintf(string(userPromptTemplate), title, lang, caption)

	apiURL := "https://api.deepseek.com/chat/completions"
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("DEEPSEEK_API_KEY environment variable is not set")
	}

	requestBody := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": string(systemPrompt),
			},
			{
				"role":    "user",
				"content": userPrompt,
			},
		},
		"stream": false,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request to DeepSeek: %w", err)
	}
	defer resp.Body.Close()

	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("DeepSeek API Response Status: %s\n", resp.Status)
	fmt.Printf("DeepSeek API Response Body: %s\n", string(responseData))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 response from DeepSeek: %s, response: %s", resp.Status, string(responseData))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(responseData, &result); err != nil {
		return "", fmt.Errorf("failed to parse response JSON: %w", err)
	}

	choices, exists := result["choices"].([]interface{})
	if !exists || len(choices) == 0 {
		return "", fmt.Errorf("no choices found in response")
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid choice format in response")
	}

	message, exists := firstChoice["message"].(map[string]interface{})
	if !exists {
		return "", fmt.Errorf("no message found in choice")
	}

	summary, exists := message["content"].(string)
	if !exists {
		return "", fmt.Errorf("no content found in message")
	}

	return summary, nil
}

func sanitizeSubtitle(subtitle string) string {
	re := regexp.MustCompile(`\s*\n\n[\s\S]*?</c>\n`)
	summarizedSrt := re.ReplaceAllString(subtitle, "\n")
	summarizedSanitized := strings.ReplaceAll(summarizedSrt, "align:start position:0%", "")
	return summarizedSanitized
}


func getSummaryFromDynamoDB(vid string, lang string) (map[string]dynamodbtypes.AttributeValue, error) {
    key := map[string]dynamodbtypes.AttributeValue{
        "PK": &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("VIDEO#%s#LANG#%s", vid, lang)},
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
        return nil, fmt.Errorf("item not found")
    }

    return result.Item, nil
}


type HandleSummaryRequestResponse struct {
	Vid         string  `json:"vid"`
	Title       string  `json:"title"`
	Lang        string  `json:"lang"`
	Status      string  `json:"status"`
	UploaderID  string  `json:"uploader_id"`
	UploadDate  string  `json:"upload_date"`
	Duration    float64 `json:"duration"`
	ChannelID   string  `json:"channel_id"`
	Category    string  `json:"category"`
	VideoLang   string  `json:"video_lang"`
	LikeCount 	int	`json:"like_count"`
}

func handleSummaryRequest(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
        return
    }

    // Parse URL query parameters
    queryParams := r.URL.Query()
    force := queryParams.Get("force") == "true"

    var requestBody struct {
        VideoID  string `json:"videoId"`
        Language string `json:"language"`
    }
    if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", requestBody.VideoID)

    videoID, err := extractVideoID(videoURL)
    if err != nil {
        http.Error(w, fmt.Sprintf("Error extracting video ID: %v", err), http.StatusBadRequest)
        return
    }

    // If force is false, try to get cached summary from DynamoDB
    if !force {
        type ContentData struct {
            Content string `json:"content"`
            Answer  string `json:"answer"`
        }

        cachedData, err := getSummaryFromDynamoDB(videoID, requestBody.Language)
		println("getFromDynamoDb",cachedData["content"], err)
		if err != nil {
			println("Error fetching from DynamoDB:", err.Error())
		}

		println("cachedData status", cachedData["status"])
		if cachedData["status"] == "processing" {
			response := GPTResponseToJson{
                Title:   cachedData["title"],
                Vid:     videoID,
                Content: "",
                Lang:    requestBody.Language,
                Answer:  "",
                Path:    cachedData["path"],
                Status:  cachedData["status"], // Add status field
				UploaderID: cachedData["uploaderId"],
				UploadDate: cachedData["uploadDate"],
				Duration: cachedData["duration"],
            }
			w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(response)
            return
		}
		if cachedData != nil {
			println("CACHED DATA - Title:", cachedData["title"])
			println("CACHED DATA - Content:", cachedData["content"])
			println("CACHED DATA - Path:", cachedData["path"])
		}
        if err == nil && cachedData != nil {
			println("CACHED DATA", cachedData)
            // Parse the JSON content from DynamoDB
            var contentData ContentData
            err := json.Unmarshal([]byte(cachedData["content"]), &contentData)
            if err != nil {
                http.Error(w, "Failed to parse content data", http.StatusInternalServerError)
                return
            }

            response := GPTResponseToJson{
                Title:   cachedData["title"],
                Vid:     videoID,
                Content: contentData.Content,
                Lang:    requestBody.Language,
                Answer:  contentData.Answer,
                Path:    cachedData["path"],
                Status:  "completed", // Add status field
				UploaderID: cachedData["uploaderId"],
				UploadDate: cachedData["uploadDate"],
				Duration: cachedData["duration"],
            }

            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(response)
            return
        }
    }
	println("NO CACHED DATA", force)
    // Get basic metadata first (synchronous)
    title, videoLanguage, uploaderID, uploadDate, duration, channelID, category, likeCounter, videoMetadataErr := getVideoMetadata(videoURL)
    if videoMetadataErr != nil {
        http.Error(w, fmt.Sprintf("Error fetching video metadata: %v", videoMetadataErr), http.StatusInternalServerError)
        return
    }

	

    // Immediate response with processing status including all metadata
    initialResponse := HandleSummaryRequestResponse{
        Vid:         videoID,
        Title:       title,
        Lang:        requestBody.Language,
        Status:      "processing",
        UploaderID:  uploaderID,
        UploadDate:  uploadDate,
        Duration:    duration,
        ChannelID:   channelID,
        Category:    category,
        VideoLang:   videoLanguage,
		LikeCount:   likeCounter,
    }
	path := convertTitleToURL(title)
	if err := pushSummaryToDynamoDB(
		initialResponse,
		"",  
		path,
	); err != nil {
		http.Error(w, fmt.Sprintf("Error pushing to DynamoDB: %v", err), http.StatusInternalServerError)
		return
	}

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(initialResponse)

    // Start async processing
    go func() {
        subtitleKey := videoID + "-caption.txt"
        subtitleSanitized, _ := fetchS3(subtitleKey)

        if subtitleSanitized == "" {
            subtitle, err := downloadSubtitle(videoURL, videoLanguage)
            if err != nil {
                log.Printf("Error downloading subtitle: %v\n", err)
                return
            }

            subtitleSanitized = sanitizeSubtitle(subtitle)

            if err := uploadToS3(subtitleSanitized, subtitleKey); err != nil {
                log.Printf("Error uploading subtitle to S3: %v\n", err)
            }
        }

        summary, err := summarizeText(subtitleSanitized, requestBody.Language, title)
        if err != nil {
            log.Printf("Error summarizing caption: %v\n", err)
            return
        }

        sanitizedSummary, err := parseFields(summary)
        if err != nil {
            log.Printf("Error sanitizing summary JSON: %v\n", err)
            return
        }

        path := convertTitleToURL(title)
		processedResponse := initialResponse
		processedResponse.Status = "done"
        if err := pushSummaryToDynamoDB(
			processedResponse,
			sanitizedSummary,
			path,
		); err != nil {
			http.Error(w, fmt.Sprintf("Error pushing to DynamoDB: %v", err), http.StatusInternalServerError)
			return
		}

		category := initialResponse.Category
		likeCounter := initialResponse.LikeCount
		
		if err := pushCategoryStatsToDynamoDB(category, likeCounter, videoID, summary); err != nil {
			log.Printf("Failed to push category stats to DynamoDB: %v", err)
		}
    }()
}
    

// Update your GPTResponseToJson struct to include Status field
type GPTResponseToJson struct {
    Title   string `json:"title,omitempty"`
    Vid     string `json:"videoId"`
    Content string `json:"content,omitempty"`
    Lang    string `json:"lang"`
    Answer  string `json:"answer,omitempty"`
    Path    string `json:"path,omitempty"`
    Status  string `json:"status"` // Mandatory field
	UploaderID string `json:"uploader_id,omitempty"`
	UploadDate string `json:"upload_date,omitempty"`
	Duration string `json:"duration,omitempty"`
}
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
    mux.HandleFunc("/summary", handleSummaryRequest)
    mux.HandleFunc("/redirects", handleGoogleRedirect)
    mux.HandleFunc("/login", handleGoogleLogin)

    // Wrap your router with the CORS handler
    handler := c.Handler(mux)

    fmt.Println("Server started at :8080 test")
    log.Fatal(http.ListenAndServe(":8080", handler))
}