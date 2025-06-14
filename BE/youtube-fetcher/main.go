package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
)

const baseURL = "https://www.youtube.com/watch?v="

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

type Caption struct {
	BaseUrl      string `json:"base_url"`
	LanguageCode string `json:"lang"`
}

type rawInfo struct {
	Microformat struct {
		PlayerMicroformatRenderer struct {
			Title             struct{ SimpleText string } `json:"title"`
			ViewCount         string                     `json:"viewCount"`
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

type FlatInfo struct {
	Title             string    `json:"title"`
	ViewCount         string    `json:"view_count"`
	ExternalChannelID string    `json:"channel_id"`
	OwnerChannelName  string    `json:"channel_name"`
	OwnerProfileURL   string    `json:"channel_url"`
	PublishDate       string    `json:"publish_date"`
	Category          string    `json:"category"`
	Captions          []Caption `json:"captions"`
}

func extractInfo(data []byte) (*FlatInfo, error) {
	var raw rawInfo
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	info := FlatInfo{
		Title:             raw.Microformat.PlayerMicroformatRenderer.Title.SimpleText,
		ViewCount:         raw.Microformat.PlayerMicroformatRenderer.ViewCount,
		ExternalChannelID: raw.Microformat.PlayerMicroformatRenderer.ExternalChannelID,
		OwnerChannelName:  raw.Microformat.PlayerMicroformatRenderer.OwnerChannelName,
		OwnerProfileURL:   raw.Microformat.PlayerMicroformatRenderer.OwnerProfileURL,
		PublishDate:       raw.Microformat.PlayerMicroformatRenderer.PublishDate,
		Category:          raw.Microformat.PlayerMicroformatRenderer.Category,
		Captions:          []Caption{},
	}

	return &info, nil
}

func fetchVideoInfo(videoID string) (*FlatInfo, error) {
	client := http.DefaultClient

	proxyStr := os.Getenv("PROXY_SERVER")
	if proxyStr != "" {
		proxyURL, err := url.Parse(proxyStr)
		if err != nil {
			log.Fatalf("invalid proxy URL: %v", err)
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

	// Extract info
	info, err := extractInfo([]byte(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error extracting info: %w", err)
	}

	// Extract captions
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

func metadataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vid := r.URL.Query().Get("vid")
	if vid == "" {
		http.Error(w, "Missing 'vid' query parameter", http.StatusBadRequest)
		return
	}

	info, err := fetchVideoInfo(vid)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching metadata: %v", err), http.StatusInternalServerError)
		return
	}

	allowedLangs := []string{"pt", "en", "es", "it", "fr"}
	filteredCaptions := []Caption{}
	for _, c := range info.Captions {
		if slices.Contains(allowedLangs, c.LanguageCode) {
			filteredCaptions = append(filteredCaptions, c)
		}
	}
	info.Captions = filteredCaptions

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func main() {
	http.HandleFunc("/metadata", metadataHandler)

	port := "6060"
	fmt.Printf("Server running on http://localhost:%s/metadata\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
