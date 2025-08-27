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
	
	// if video is WExJh9b9e2E id return the json above

	itShouldBegSrcc0oA6Q4 := []byte(`{
		"title": "Jornalista da GloboNews revela maior medo do Supremo! Assista",
		"view_count": "241573",
		"length_seconds": "581",
		"channel_id": "UC84asuWqcrFqEtWqSCtS85Q",
		"channel_name": "Deltan Dallagnol",
		"channel_url": "http://www.youtube.com/@DeltanDallagnolOficial",
		"publish_date": "2025-06-27T07:35:11-07:00",
		"category": "News & Politics",
		"captions": [
			{
				"base_url": "https://www.youtube.com/api/timedtext?v=gSrcc0oA6Q4&ei=HvBgaK2GDe2P-LAPh7mtsA4&caps=asr&opi=112496729&exp=xpe&xoaf=5&hl=pt&ip=0.0.0.0&ipbits=0&expire=1751208590&sparams=ip,ipbits,expire,v,ei,caps,opi,exp,xoaf&signature=37D3BBE06906CBC8948A630882804AAA229088BD.79E1E24DE53BFC76079F288B67710F55D1E66368&key=yt8&kind=asr&lang=pt&variant=punctuated",
				"lang": "pt"
			}
		]
	}`)

	if vid == "gSrcc0oA6Q4" {
		err = json.Unmarshal(itShouldBegSrcc0oA6Q4, &info)
		if err != nil {
			log.Fatal("error unmarshalling JSON:", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
		return
	}
	

	switch method {
		case "downsub":
			// info, err = FetchUsingDownsub(vid) // futura função
			downSubReturn, err := FetchMetadataFromDownsub(vid)
			if (err != nil) {
				http.Error(w, fmt.Sprintf("Error fetching metadata: %v", err), http.StatusInternalServerError)
				return
			}
			println("📤 downsub response CAT", downSubReturn.Data.Metadata.Category)
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
