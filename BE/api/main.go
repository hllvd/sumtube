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
				I will provide a title, language and captions as input.  
				
				Your task is to generate a JSON output with three fields: $content, $lang, and $answer. Here's what each field should contain:  
				
				- ╔$content:
				  - A summarized version of the text in Markdown format.  
				  - Use **bold**, *italic*, lists, and headers for emphasis where needed. Use \n for line breaks.  
				  - If you need to use quotes, wrap the sentence with underscores (_like this_) instead of regular quotes.  
				  - If more text is required, you may add an extra paragraph, but it should not exceed 300 words.  
				  - If the title is a listicle (e.g., "Top 10 Ways to..."), format the summary as a list.
				  - You can link the subject of the summary to its context in the video by specifying the exact timestamp. For example: "[Elon Musk](00:10:51) said that...". This will link to minute 10, second 51 of the text.
				  - If the summary is in list format, start with a **bold subtitle**, followed by a sentence that includes a timestamp linking to the specific moment in the video. Example: "**Key Takeaways**\n\n[At 10:51](00:10:51), Elon Musk said...". Then, present the summarized points in a clear and structured list.
				  - Keep it concise and structured without stating that it is a summary
				  - When finish the content of this field, please use ╗
				
				- ╔$lang:
				  - Detect and output the language code of the text.  
				  - Example: "en", "pt", "es", "it", "fr", "de".
				  - When finish the content of this field, please use ╗  
				
				- ╔$answer:
				  - If the title is a question, provide a concise answer (maximum 32 words). This is important!  
				  - If the title is not a question, rephrase it starting with "When", "How", or "How to" so that it can be answered.
				  - When finish the content of this field, please use ╗
				
				** The final expected output would be something like: ** 
				╔$content:[Summarized text here] ╗
				╔$lang:[lang here] ╗
				╔$answer:[Answer here] ╗
				}`),
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

type GPTResponseToJson struct {
    Title   string `json:"title"`
    Vid     string `json:"vid"`
    Content string `json:"content"`
    Lang    string `json:"lang"`
    Answer  string `json:"answer"`
    Path    string `json:"path"` // Adding path since it's available
}

func getFromDynamoDb(language string, videoID string) (map[string]string, error) {
    compositeKey := fmt.Sprintf("%s#%s", videoID, language)
    
    result, err := dynamoDBClient.GetItem(context.Background(), &dynamodb.GetItemInput{
        TableName: aws.String(dynamoDBTableName),
        Key: map[string]dynamodbtypes.AttributeValue{
            "id": &dynamodbtypes.AttributeValueMemberS{Value: compositeKey},
        },
    })
    if err != nil {
        return nil, fmt.Errorf("failed to get item from DynamoDB: %w", err)
    }
    
    if result.Item == nil {
        return nil, nil // Item not found
    }
    
    dataAttr, ok := result.Item["data"].(*dynamodbtypes.AttributeValueMemberM)
    if !ok {
        return nil, fmt.Errorf("invalid data format in DynamoDB item")
    }
    
    contentAttr, ok := dataAttr.Value["content"].(*dynamodbtypes.AttributeValueMemberS)
    if !ok {
        return nil, fmt.Errorf("content field missing or invalid")
    }
    
    titleAttr, ok := dataAttr.Value["title"].(*dynamodbtypes.AttributeValueMemberS)
    if !ok {
        return nil, fmt.Errorf("title field missing or invalid")
    }
    
    pathAttr, ok := dataAttr.Value["path"].(*dynamodbtypes.AttributeValueMemberS)
    if !ok {
        return nil, fmt.Errorf("path field missing or invalid")
    }
    
    return map[string]string{
        "content": contentAttr.Value,
        "title":   titleAttr.Value,
        "path":    pathAttr.Value,
    }, nil
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
        VideoID string `json:"videoId"`
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

        cachedData, err := getFromDynamoDb(requestBody.Language, videoID)
		if err == nil && cachedData != nil {
			// Parse the JSON content from DynamoDB
			var contentData ContentData
			err := json.Unmarshal([]byte(cachedData["content"]), &contentData)
			if err != nil {
				// Handle JSON parsing error
				http.Error(w, "Failed to parse content data", http.StatusInternalServerError)
				return
			}

			// Create the structured response
			response := GPTResponseToJson{
				Title:   cachedData["title"],
				Vid:     videoID,
				Content: contentData.Content,
				Lang:    requestBody.Language,
				Answer:  contentData.Answer,
				Path:    cachedData["path"], // Include path if needed
			}

			// Return the response as JSON
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
    }

    title, videoLanguage, videoMetadataErr := getVideoMetadata(videoURL)
    if videoMetadataErr != nil {
        http.Error(w, fmt.Sprintf("Error fetching video metadata: %v", videoMetadataErr), http.StatusInternalServerError)
        return
    }

	subtitleKey := videoID + "-caption.txt"
	subtitleSanitized, s3Err := fetchS3(subtitleKey) 
	println("subtitleSanitized: ", subtitleSanitized)
	println("s3Err: ", s3Err)
	if subtitleSanitized == "" {
		subtitle, err := downloadSubtitle(videoURL, videoLanguage)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error downloading subtitle: %v", err), http.StatusInternalServerError)
			return
		}

		subtitleSanitized = sanitizeSubtitle(subtitle)
		
		go func() {
			if err := uploadToS3(subtitleSanitized, subtitleKey); err != nil {
				log.Printf("Error uploading subtitle to S3: %v\n", err)
			}
		}()
	}
	
    summary, err := summarizeText(subtitleSanitized, requestBody.Language, title)
    if err != nil {
        http.Error(w, fmt.Sprintf("Error summarizing caption: %v", err), http.StatusInternalServerError)
        return
    }
    fmt.Printf("\n Debug summary: %+v\n", summary)

    sanitizedSummary, err := parseFields(summary)
    if err != nil {
        http.Error(w, fmt.Sprintf("Error sanitizing summary JSON: %v", err), http.StatusInternalServerError)
        return
    }

    fmt.Printf("\n Debug sanitizedSummary: %+v\n", sanitizedSummary)

	path := convertTitleToURL(title)
	if err := pushToDynamoDB(videoID, requestBody.Language, title, sanitizedSummary, path); err != nil {
		http.Error(w, fmt.Sprintf("Error pushing to DynamoDB: %v", err), http.StatusInternalServerError)
		return
	}

    // Create a variable to hold the unmarshaled summary data
    var summaryData map[string]string
    if err := json.Unmarshal([]byte(sanitizedSummary), &summaryData); err != nil {
        http.Error(w, fmt.Sprintf("Error parsing sanitized summary JSON: %v", err), http.StatusInternalServerError)
        return
    }

    // Create the response struct with all fields
    response := GPTResponseToJson{
        Title:    title,
        Vid:      videoID,
        Content:  summaryData["content"],  // Will be mapped to "content" in JSON
        Lang: summaryData["lang"], // Will be mapped to "lang" in JSON
        Answer:   summaryData["answer"],
		Path: path,
    }

    // Print debug information
    fmt.Printf("Debug response: %+v\n", response)

    // Convert to JSON
    jsonResponse, err := json.Marshal(response)
    if err != nil {
        http.Error(w, fmt.Sprintf("Error encoding JSON response: %v", err), http.StatusInternalServerError)
        return
    }

    fmt.Printf("Debug jsonResponse: %s\n", jsonResponse)
    
    w.Header().Set("Content-Type", "application/json")
    w.Write(jsonResponse)
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
	fmt.Println("Server started at :8080 test")
	log.Fatal(http.ListenAndServe(":8080", nil))
}