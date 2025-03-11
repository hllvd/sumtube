package main

import (
	"fmt"
	"io/ioutil"
	"log"
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
}
