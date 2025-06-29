package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const baseURL = "https://www.youtube.com/watch?v="



type rawInfo struct {
	Microformat struct {
		PlayerMicroformatRenderer struct {
			Title             struct{ SimpleText string } `json:"title"`
			ViewCount         string                     `json:"viewCount"`
			LengthSeconds     string                     `json:"lengthSeconds"`
			ExternalChannelID string                     `json:"externalChannelId"`
			OwnerChannelName  string                     `json:"ownerChannelName"`
			OwnerProfileURL   string                     `json:"ownerProfileUrl"`
			PublishDate       string                     `json:"publishDate"`
			Category          string                     `json:"category"`
		} `json:"playerMicroformatRenderer"`
	} `json:"microformat"`
	Captions struct {
		PlayerCaptionsTracklistRenderer struct {
			CaptionTracks []struct {
				BaseURL string `json:"baseUrl"`
			} `json:"captionTracks"`
		} `json:"playerCaptionsTracklistRenderer"`
	} `json:"captions"`
}

type ytInitialPlayerResponse struct {
	Captions struct {
		PlayerCaptionsTracklistRenderer struct {
			CaptionTracks []struct {
				BaseUrl      string `json:"baseUrl"`
				LanguageCode string `json:"languageCode"`
			} `json:"captionTracks"`
		} `json:"playerCaptionsTracklistRenderer"`
	} `json:"captions"`
}

func extractInfo(data []byte) (*YoutubeMetadataResponse, error) {
	var raw rawInfo
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	info := YoutubeMetadataResponse{
		Title:             raw.Microformat.PlayerMicroformatRenderer.Title.SimpleText,
		ViewCount:         raw.Microformat.PlayerMicroformatRenderer.ViewCount,
		LengthSeconds:     raw.Microformat.PlayerMicroformatRenderer.LengthSeconds,
		ExternalChannelID: raw.Microformat.PlayerMicroformatRenderer.ExternalChannelID,
		OwnerChannelName:  raw.Microformat.PlayerMicroformatRenderer.OwnerChannelName,
		ChannelUrl:   	   raw.Microformat.PlayerMicroformatRenderer.OwnerProfileURL,
		PublishDate:       raw.Microformat.PlayerMicroformatRenderer.PublishDate,
		Category:          raw.Microformat.PlayerMicroformatRenderer.Category,
		Captions:          []Caption{},
	}

	return &info, nil
}

func FetchDirectly(videoID string) (*YoutubeMetadataResponse, error) {
	client := http.DefaultClient

	proxyStr := os.Getenv("PROXY_SERVER")
	if proxyStr != "" {
		proxyURL, err := url.Parse(proxyStr)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %v", err)
		}
		client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	}

	resp, err := client.Get(baseURL + videoID)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch video page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response: %w", err)
	}

	content := string(body)

	split := strings.Split(content, "ytInitialPlayerResponse = ")
	if len(split) < 2 {
		return nil, fmt.Errorf("ytInitialPlayerResponse not found")
	}
	split = strings.Split(split[1], ";</script>")
	if len(split) < 1 {
		return nil, fmt.Errorf("ytInitialPlayerResponse end not found")
	}

	jsonData := split[0]

	info, err := extractInfo([]byte(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error extracting info: %w", err)
	}

	var captions ytInitialPlayerResponse
	if err := json.Unmarshal([]byte(jsonData), &captions); err != nil {
		return nil, fmt.Errorf("error parsing captions: %w", err)
	}

	for _, c := range captions.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks {
		info.Captions = append(info.Captions, Caption{
			BaseUrl:      c.BaseUrl,
			LanguageCode: c.LanguageCode,
		})
	}

	return info, nil
}
