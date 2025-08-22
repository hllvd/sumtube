package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)
type DownsubResponse struct {
	Status string 	 `json:"status"`
	Data DownsubData `json:"data"`
}
type DownsubData struct {
	State string `json:"state"`
	Title string `json:"title"`
	Duration int `json:"duration"`
	Metadata Metadata `json:"metadata"`
	Subtitles []Subtitles `json:"subtitles"`
	TranslatedSubtitles []Subtitles `json:"translatedSubtitles"`
}

type Subtitles struct {
	Language string `json:"language"`
	Formats []Format `json:"formats"`
}

type Format struct {
	Format 	string `json:"format"`
	Url 	string `json:"url"`
}

type Metadata struct {
	Author string `json:"author"`
	ChannelId string `json:"channelId"`
	ChannelUrl string `json:"channelUrl"`
	Description string `json:"description"`
	PublishDate string `json:"publishDate"`
	Category string `json:"category"`
	ViewCount json.Number `json:"viewCount"`
}

var (
	downsubAPIKey string
	downsubBaseURL string
)

var httpClient = &http.Client{} 

func init() {
	downsubAPIKey = os.Getenv("DOWNSUB_API_KEY")
	downsubBaseURL = os.Getenv("DOWNSUB_BASE_URL")

	if downsubAPIKey == "" {
		panic("DOWNSUB_API_KEY environment variable is not set")
	}
	if downsubBaseURL == "" {
		panic("DOWNSUB_BASE_URL environment variable is not set")
	}
}


func FetchMetadataFromDownsub(videoID string) (*DownsubResponse, error) {
	payload := map[string]string{
		"url": fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", downsubBaseURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+downsubAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("downsub returned status %d: %s", resp.StatusCode, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var info DownsubResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return &info, nil
}



func convertDownSubResponseToFlatResponse(downsubInfo *DownsubResponse) *YoutubeMetadataResponse {
	langMap := map[string]string{
		"portuguese": "pt",
		"english":    "en",
		"spanish":    "es",
		"italian":    "it",
		"french":     "fr",
	}

	captions := make([]Caption, 0)

	for _, subtitle := range downsubInfo.Data.Subtitles {
		langName := strings.ToLower(strings.Split(subtitle.Language, " ")[0])
		langCode, ok := langMap[langName]
		if !ok {
			continue
		}
		for _, format := range subtitle.Formats {
			if format.Format == "srt" {
				captions = append(captions, Caption{
					BaseUrl:      format.Url,
					LanguageCode: langCode,
				})
				break
			}
		}
	}

	vcStr := downsubInfo.Data.Metadata.ViewCount.String()
	return &YoutubeMetadataResponse{
		Title:             downsubInfo.Data.Title,
		LengthSeconds:     fmt.Sprintf("%d", downsubInfo.Data.Duration),
		OwnerChannelName:  downsubInfo.Data.Metadata.Author,
		ExternalChannelID: downsubInfo.Data.Metadata.ChannelId,
		ChannelUrl:        downsubInfo.Data.Metadata.ChannelUrl,
		PublishDate:       downsubInfo.Data.Metadata.PublishDate,
		Category:          downsubInfo.Data.Metadata.Category,
		ViewCount:         vcStr,
		Captions:          captions,
	}
}




