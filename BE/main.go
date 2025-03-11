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
	"strings"

	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const tempDir = "/tmp"
const bucketName = "sumtube"

var s3Client *s3.Client

// Initialize AWS SDK and create an S3 client
func init() {
	fmt.Println("Initializing AWS SDK...")
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"), // Specify the region here
	)
	if err != nil {
		log.Fatalf("unable to load AWS SDK config, %v\n", err)
	}
	s3Client = s3.NewFromConfig(cfg)
	fmt.Println("AWS SDK initialized successfully.")
}

// downloadSubtitle downloads the subtitle from a YouTube video using yt-dlp
func downloadSubtitle(videoURL string) (string, error) {
	// Build the output file template
	outputTemplate := filepath.Join(tempDir, "%(id)s.%(ext)s")

	fmt.Println("Downloading subtitles using yt-dlp...")
	// Run yt-dlp to download auto-subtitles in VTT format
	cmd := exec.Command("yt-dlp", "--skip-download", "--write-auto-sub", "--sub-lang", "en", "-o", outputTemplate, videoURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run yt-dlp: %w", err)
	}

	// Extract the file name (assuming the video ID will be used as the name)
	subtitleFilePath := filepath.Join(tempDir, "*.vtt")
	files, err := filepath.Glob(subtitleFilePath)
	if err != nil {
		return "", fmt.Errorf("error finding subtitle file: %w", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no subtitle files found")
	}

	// Read subtitle file
	subtitleFilePath = files[0]
	fmt.Println("Reading subtitle file:", subtitleFilePath)
	data, err := ioutil.ReadFile(subtitleFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle file: %w", err)
	}

	// Return subtitle text
	return string(data), nil
}

// uploadToS3 uploads the subtitle to S3
func uploadToS3(content string, key string) error {
	fmt.Println("Uploading subtitle to S3...")
	_, err := s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   strings.NewReader(content),
		ContentType: aws.String("text/plain"),
		ACL:    types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}
	fmt.Println("Subtitle uploaded to S3 successfully.")
	return nil
}

// summarizeText sends a request to DeepSeek API to summarize the text
func summarizeText(caption string) (string, error) {
	// DeepSeek API URL
	apiURL := "https://api.deepseek.com/chat/completions"

	// Read the API key from the environment variable
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("DEEPSEEK_API_KEY environment variable is not set")
	}

	// Prepare the request body
	requestBody := map[string]interface{}{
		"model": "deepseek-chat", // Specify the model
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a helpful assistant.",
			},
			{
				"role":    "user",
				"content": fmt.Sprintf("I would like to summarize this text into less than 200 words: %s", caption),
			},
		},
		"stream": false, // Disable streaming for a single response
	}

	// Marshal the request body into JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey) // Use the API key from the environment variable

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request to DeepSeek: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Debugging: Print the response status and body
	fmt.Printf("DeepSeek API Response Status: %s\n", resp.Status)
	fmt.Printf("DeepSeek API Response Body: %s\n", string(responseData))

	// Check for successful response (e.g., 200 status code)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 response from DeepSeek: %s, response: %s", resp.Status, string(responseData))
	}

	// Parse the response JSON
	var result map[string]interface{}
	if err := json.Unmarshal(responseData, &result); err != nil {
		return "", fmt.Errorf("failed to parse response JSON: %w", err)
	}

	// Extract the summarized text from the response
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

func main() {
	// Example YouTube video URL
	videoURL := "https://www.youtube.com/watch?v=dREhPVW5tb8" // replace with a valid YouTube URL

	// Download subtitle
	fmt.Println("Starting subtitle download...")
	subtitle, err := downloadSubtitle(videoURL)
	if err != nil {
		log.Fatalf("Error downloading subtitle: %v\n", err)
	}

	// Print subtitle content (for debugging purposes)
	fmt.Println("Subtitles: \n", subtitle)

	// Create S3 key for the subtitle file (using video ID)
	videoID := "dREhPVW5tb8" // Replace with actual video ID extraction logic if needed
	subtitleKey := videoID + "-caption.txt"

	// Upload subtitle to S3
	if err := uploadToS3(subtitle, subtitleKey); err != nil {
		log.Fatalf("Error uploading subtitle to S3: %v\n", err)
	}

	// Summarize the caption using DeepSeek
	fmt.Println("Sending caption to DeepSeek for summarization...")
	summary, err := summarizeText(subtitle)
	if err != nil {
		log.Fatalf("Error summarizing caption: %v\n", err)
	}

	// Output the summarized text
	fmt.Println("Summarized Caption: \n", summary)
}
