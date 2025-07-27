package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"

	"github.com/google/uuid"
)
type YoutubeMetadataResponse struct {
	Title             string    `json:"title"`
	ViewCount         string    `json:"view_count"`
	LengthSeconds     string    `json:"length_seconds"`
	ExternalChannelID string    `json:"channel_id"`
	OwnerChannelName  string    `json:"channel_name"`
	ChannelUrl   	  string    `json:"channel_url"`
	PublishDate       string    `json:"publish_date"`
	Category          string    `json:"category"`
	Captions          []Caption `json:"captions"`
}

type Caption struct {
	BaseUrl      string `json:"base_url"`
	LanguageCode string `json:"lang"`
}

func metadataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vid := r.URL.Query().Get("vid")
	method := os.Getenv("METADATA_METHOD")

	if vid == "" {
		http.Error(w, "Missing 'vid' query parameter", http.StatusBadRequest)
		return
	}

	var info *YoutubeMetadataResponse
	var err error

	switch method {
		case "downsub":
			// info, err = FetchUsingDownsub(vid) // futura função
			downSubReturn, err := FetchMetadataFromDownsub(vid)
			if (err != nil) {
				http.Error(w, fmt.Sprintf("Error fetching metadata: %v", err), http.StatusInternalServerError)
				return
			}
			
			info = convertDownSubResponseToFlatResponse(downSubReturn)
			//fmt.Printf("📤 Final response: %+v\n", info)
			//http.Error(w, "downsub not implemented", http.StatusNotImplemented)
		default:
			info, err = FetchDirectly(vid)
	}

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
	fmt.Println("UUID Example:", uuid.NewString())
	http.HandleFunc("/metadata", metadataHandler)

	port := "6060"
	fmt.Printf("Server running on http://localhost:%s/metadata\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
