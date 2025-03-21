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

	//fmt.Println("data : ", string(data))
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
                I would pass a title and the captions as input.
                    If the title is 'Listicle Title', please make a list with the answers along side the content.
                    I need 3 fields for output: $content, $lang and $answer. I will explain what I would expect for these fields:
                    - $content:
                    -- The $content would be the summarized text of the text in markdown to emphasize important things if needed. 
                    -- Allowed markdown: bold, italic, code, link, list, and header.
                    -- A good snippet to put as a seo description which is the first 150 words of the content.
                    -- If more text is required, please add an extra paragraph with no more than 300 words is necessary.
                    -- The output of this field would be $content: [summarized text here]
                    
                    - $lang:
                    -- Please use the current language of the text to output the summary.
                    -- The output of the lang would 'language:' following by the current language. Don't wrap with quotes, put only the result
                    
                    $answer: 
                    -- This should be the answer of the title of the video if the title is a question.
                    -- Limit this field's answer to no more than 32 words. This is important!
                    -- If the title have no question, please put the word 'When', 'How' in the begin of the title.
                    
                    * I would like the output THE FIELDS separated by \n---\n file like this example:
                    $content: This is the content
                    ---
                    $lang: en|pt|es|it|fr|du
                    ---
                    $answer: This is the answer
             
                   `, lang),
			},
			{
				"role":    "user",
				"content": fmt.Sprintf("I would like to summarize this text with title `[when|how|how to] %s ?` in %s language, the caption is: %s", title, lang, caption),
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

type Output struct {
	Content string `json:"content"`
	Lang    string `json:"lang"`
	Answer  string `json:"answer"`
}

// Function to parse the input and convert it to JSON
func parseInputToJSON(input string) (string, error) {
	// Initialize the struct
	var output Output

	// Split the input by the separator
	parts := strings.Split(input, "\n---\n")

	// Iterate over each part to extract the fields
	for _, part := range parts {
		if strings.HasPrefix(part, "$content:") {
			output.Content = strings.TrimSpace(strings.TrimPrefix(part, "$content:"))
		} else if strings.HasPrefix(part, "$lang:") {
			output.Lang = strings.TrimSpace(strings.TrimPrefix(part, "$lang:"))
		} else if strings.HasPrefix(part, "$answer:") {
			output.Answer = strings.TrimSpace(strings.TrimPrefix(part, "$answer:"))
		}
	}

	// Marshal the struct into JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

func validationMessageBody(summary string) bool {
	// Use raw string (backticks) for the regex pattern
	pattern := `^\$content:(.+?)\n---\n\$lang:(.+?)\n---\n\$answer:(.+?)$`
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(summary)
	if (len(matches) > 0) {return true}
    return false
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

    if validationMessageBody(summary) {
		fmt.Println("Validation successful!")
	} else {
		fmt.Println("Validation failed!")
	}

	// Debugging: Print the cleaned summary string
	fmt.Println("Cleaned summary string: \n\n", summary)

	path := convertTitleToURL(title)
	if err := pushToDynamoDB(videoID, language, title, summary, path); err != nil {
		http.Error(w, fmt.Sprintf("Error pushing to DynamoDB: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse the summary into JSON
	jsonParsed, err := parseInputToJSON(summary)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing summary to JSON: %v", err), http.StatusInternalServerError)
		return
	}

	// Set the response content type to JSON
	w.Header().Set("Content-Type", "application/json")

	// Write the JSON response
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(jsonParsed))
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
    http.HandleFunc("/redirects", handleGoogleRedirect) // OAuth2 redirect endpoint
	http.HandleFunc("/login", handleGoogleLogin)      // Start OAuth2 flow
	fmt.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}