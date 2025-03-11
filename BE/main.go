package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const tempDir = "/tmp"

func downloadSubtitle(videoURL string) (string, error) {
	// Build the output file template
	outputTemplate := filepath.Join(tempDir, "%(id)s.%(ext)s")

	// Run yt-dlp to download auto-subtitles in VTT format (or use youtube-dl if you prefer)
	cmd := exec.Command("yt-dlp", "--skip-download", "--write-auto-sub", "--sub-lang", "en", "-o", outputTemplate, videoURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run yt-dlp: %w", err)
	}

	// Extract the file name (assuming the video ID will be used as the name)
	// You can modify this part if needed.
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
	data, err := ioutil.ReadFile(subtitleFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle file: %w", err)
	}

	// Return subtitle text
	return string(data), nil
}

func main() {
	// Example YouTube video URL
	videoURL := "https://www.youtube.com/watch?v=dREhPVW5tb8" // replace with a valid YouTube URL

	// Download subtitle
	subtitle, err := downloadSubtitle(videoURL)
	if err != nil {
		log.Fatalf("Error downloading subtitle: %v\n", err)
	}

	// Print subtitle content
	fmt.Println("Subtitles: \n", subtitle)
}
