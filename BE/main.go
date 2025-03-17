package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuth2 configuration for Google
var googleOAuthConfig = &oauth2.Config{
	ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),     // Set your Google OAuth Client ID
	ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"), // Set your Google OAuth Client Secret
	RedirectURL:  "http://localhost:8080/redirect",  // Redirect URL must match the one configured in Google Console
	Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
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
	Title    string `json:"title"`
	Language string `json:"language"`
}

func getVideoMetadata(videoURL string) (string, string, error) {
	cmd := exec.Command("yt-dlp", "--dump-json", videoURL)
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to run yt-dlp: %w", err)
	}

	var metadata VideoMetadata
	if err := json.Unmarshal(output, &metadata); err != nil {
		return "", "", fmt.Errorf("failed to parse metadata: %w", err)
	}

	return metadata.Title, metadata.Language, nil
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
	outputTemplate := filepath.Join(tempDir, "%(id)s.%(ext)s")

	fmt.Println("Downloading subtitles using yt-dlp...")
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
	fmt.Println("searchQuery : ", searchQuery)
	subtitleFilePath := filepath.Join(tempDir, searchQuery)
	files, err := filepath.Glob(subtitleFilePath)
	if err != nil {
		return "", fmt.Errorf("error finding subtitle file: %w", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no subtitle files found")
	}

	subtitleFilePath = files[0]
	fmt.Println("Reading subtitle file:", subtitleFilePath)
	data, err := ioutil.ReadFile(subtitleFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle file: %w", err)
	}

	fmt.Println("data : ", string(data))
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

func pushToDynamoDB(videoID string, lang string, title string, summary string, path string) error {
	compositeKey := fmt.Sprintf("%s#%s", videoID, lang)
	item := map[string]dynamodbtypes.AttributeValue{
		"id": &dynamodbtypes.AttributeValueMemberS{Value: compositeKey},
		"data": &dynamodbtypes.AttributeValueMemberM{
			Value: map[string]dynamodbtypes.AttributeValue{
				"content": &dynamodbtypes.AttributeValueMemberS{Value: summary},
				"title":   &dynamodbtypes.AttributeValueMemberS{Value: title},
				"path":    &dynamodbtypes.AttributeValueMemberS{Value: path},
			},
		},
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

func summarizeText(caption string, lang string, title string) (string, error) {
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
				"content": fmt.Sprintf(`You are a helpful assistant. 
                    Please use the current language of the text to output the summary. 
                    If the title of the text has a question, need to answer that question in the content and in the 'answer' property of the json. 
                    The output would be a json file like this example: {content:"summarized text here", lang:"%s", answer:"answer here"}.
                   `, lang),
			},
			{
				"role":    "user",
				"content": fmt.Sprintf("I would like to summarize this text with title %s in %s language, into less than 200 words: %s", title, lang, caption),
			},
		},
        "type": "json_object",
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

func handleSummaryRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var requestBody struct {
		VideoID string `json:"videoId"`
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

	title, language, err := getVideoMetadata(videoURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching video metadata: %v", err), http.StatusInternalServerError)
		return
	}

	subtitle, err := downloadSubtitle(videoURL, language)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error downloading subtitle: %v", err), http.StatusInternalServerError)
		return
	}

	subtitleSanitized := sanitizeSubtitle(subtitle)
	subtitleKey := videoID + "-caption.txt"

	go func() {
		if err := uploadToS3(subtitleSanitized, subtitleKey); err != nil {
			log.Printf("Error uploading subtitle to S3: %v\n", err)
		}
	}()

	summary, err := summarizeText(subtitleSanitized, language, title)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error summarizing caption: %v", err), http.StatusInternalServerError)
		return
	}

	// Debugging: Print the raw summary string
	fmt.Println("Raw summary string:", summary)

	// Clean the summary string by removing the Markdown code block wrapper
	cleanedSummary := strings.TrimPrefix(summary, "```json\n")
	cleanedSummary = strings.TrimSuffix(cleanedSummary, "\n```")
	cleanedSummary = strings.TrimSpace(cleanedSummary)

	// Debugging: Print the cleaned summary string
	fmt.Println("Cleaned summary string:", cleanedSummary)

	// Parse the cleaned summary string into a structured JSON object
	var summaryResult struct {
		Content string `json:"content"`
		Answer  string `json:"answer"`
		Lang    string `json:"lang"`
	}
	if err := json.Unmarshal([]byte(cleanedSummary), &summaryResult); err != nil {
		http.Error(w, fmt.Sprintf("Error parsing summary JSON: %v", err), http.StatusInternalServerError)
		return
	}

	path := convertTitleToURL(title)
	if err := pushToDynamoDB(videoID, language, title, summary, path); err != nil {
		http.Error(w, fmt.Sprintf("Error pushing to DynamoDB: %v", err), http.StatusInternalServerError)
		return
	}

	// Return the structured JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"content": summaryResult.Content,
		"answer":  summaryResult.Answer,
		"lang":    summaryResult.Lang,
	})
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

func main() {
	http.HandleFunc("/summary", handleSummaryRequest)
    http.HandleFunc("/redirect", handleGoogleRedirect) // OAuth2 redirect endpoint
	http.HandleFunc("/login", handleGoogleLogin)      // Start OAuth2 flow
	fmt.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}