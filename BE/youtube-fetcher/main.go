package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

  defer file.Close()

  _, err = io.Copy(file, resp.Body)
  if err != nil {
    return fmt.Errorf("unable to write file: %w", err)
  }

  return nil
}

func listVideoCaptions(videoID string) ([]Caption, error) {
  urlApi := baseURL + videoID // + "&cc_load_policy=1"
  println("url : ", urlApi)
  proxyUrl, _ := url.Parse("http://api06c9cad29d4edd53:RNW78Fm5@res.proxy-seller.com:10017")
  client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
  resp, err := client.Get(baseURL + videoID)
  if err != nil {
    return nil, fmt.Errorf("unable to download video page: %w", err)
  }

  defer resp.Body.Close()

  content, err := io.ReadAll(resp.Body)
  if err != nil {
    return nil, fmt.Errorf("unable to read response body: %w", err)
  }

  pageContent := string(content)

  // Find ytInitialPlayerResponse variable
  pageContentSplited := strings.Split(pageContent, "ytInitialPlayerResponse = ")
  if len(pageContentSplited) < 2 {
    return nil, fmt.Errorf("unable to find ytInitialPlayerResponse variable")
  }

  // Find the end of the variable
  pageContentSplited = strings.Split(pageContentSplited[1], ";</script>")
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
 if len(os.Args) < 2 {
  log.Fatalf("usage: %s <videoID>", filepath.Base(os.Args[0]))
 }

 videoID := os.Args[1]

 captions, err := listVideoCaptions(videoID)
 if err != nil {
  log.Fatalf("unable to list video captions: %v", err)
 }

 for _, caption := range captions {
  if caption.LanguageCode == "en" {
   err := caption.Download(fmt.Sprintf("%s.xml", videoID))
   if err != nil {
    log.Printf("unable to download caption: %v", err)
   }
  }
 }
}