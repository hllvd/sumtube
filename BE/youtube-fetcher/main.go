package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
 BaseUrl      string
 LanguageCode string
}

func (c *Caption) Download(targetPath string) error {
 resp, err := http.Get(c.BaseUrl)
 if err != nil {
  return fmt.Errorf("unable to download caption: %w", err)
 }

 defer resp.Body.Close()

 file, err := os.Create(targetPath)
 if err != nil {
  return fmt.Errorf("unable to create file: %w", err)
 }
 println(file)
 defer file.Close()

 _, err = io.Copy(file, resp.Body)
 if err != nil {
  return fmt.Errorf("unable to write file: %w", err)
 }

 return nil
}

func listVideoCaptions(videoID string) ([]Caption, error) {
 resp, err := http.Get(baseURL + videoID)
 if err != nil {
  return nil, fmt.Errorf("unable to download video page: %w", err)
 }

 defer resp.Body.Close()

 content, err := io.ReadAll(resp.Body)
 println("content",content)
 if err != nil {
  return nil, fmt.Errorf("unable to read response body: %w", err)
 }

 pageContent := string(content)

 // Find ytInitialPlayerResponse variable
 pageContentSplited := strings.Split(pageContent, "ytInitialPlayerResponse = ")
 println("pageContentSplited",pageContentSplited)
 if len(pageContentSplited) < 2 {
  return nil, fmt.Errorf("unable to find ytInitialPlayerResponse variable")
 }

 // Find the end of the variable
 pageContentSplited = strings.Split(pageContentSplited[1], ";</script>")
 println("pageContentSplited",pageContentSplited)
 if len(pageContentSplited) < 2 {
  return nil, fmt.Errorf("unable to find the end of the ytInitialPlayerResponse variable")
 }

 ytInitialPlayerResponse := ytInitialPlayerResponse{}
 err = json.Unmarshal([]byte(pageContentSplited[0]), &ytInitialPlayerResponse)
 if err != nil {
  return nil, fmt.Errorf("unable to unmarshal ytInitialPlayerResponse: %w", err)
 }

 captions := make([]Caption, 0, len(ytInitialPlayerResponse.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks))
 for _, caption := range ytInitialPlayerResponse.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks {
  captions = append(captions, Caption{
   BaseUrl:      caption.BaseUrl,
   LanguageCode: caption.LanguageCode,
  })
 }

 return captions, nil
}

func main() {
 println("init")
 if len(os.Args) < 2 {
  log.Fatalf("usage: %s <videoID>", filepath.Base(os.Args[0]))
 }
 println("after len")
 videoID := os.Args[1]
 println("videoID",videoID)
 captions, err := listVideoCaptions(videoID)
 println("captions",captions)
 if err != nil {
  log.Fatalf("unable to list video captions: %v", err)
 }
 
 allowedLangs := []string{"pt", "en", "es", "it", "fr"}
 for _, caption := range captions {
  captionLang := caption.LanguageCode
   println("captionLang",captionLang)
  
   if slices.Contains(allowedLangs, captionLang) {
   err := caption.Download(fmt.Sprintf("%s.xml", videoID))
   if err != nil {
    log.Printf("unable to download caption: %v", err)
   }
  }
 }
}