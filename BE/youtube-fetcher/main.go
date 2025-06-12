package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type Transcript struct {
	XMLName xml.Name `xml:"transcript"`
	Texts   []Text   `xml:"text"`
}

type Text struct {
	Start string `xml:"start,attr"`
	Dur   string `xml:"dur,attr"`
	Body  string `xml:",chardata"`
}

func main() {

	u := launcher.New().
    Headless(true).
    Set("--mute-audio").
    Set("--disable-blink-features", "AutomationControlled").
    Set("--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
        "(KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36").
    MustLaunch()


	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("")
	defer page.MustClose()

	var candidateURLs []string

	router := browser.HijackRequests()
	router.Add("*", proto.NetworkResourceTypeXHR, func(ctx *rod.Hijack) {
		requestURL := ctx.Request.URL().String()
		if strings.Contains(requestURL, "timedtext") {
			fmt.Println("Found timedtext URL:", requestURL)
			candidateURLs = append(candidateURLs, requestURL)
		}
		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	})
	
	go router.Run()

	page.MustNavigate("https://www.youtube.com/watch?v=mT0RNrTDHkI")
	page.MustElement("video")
	page.MustElement(".ytp-subtitles-button.ytp-button").MustClick()

	time.Sleep(15 * time.Second)

	if len(candidateURLs) == 0 {
		fmt.Println("⚠️ No timedtext URLs captured")
		return
	}

	var subtitleData []byte
	var validURL string

	// Try to fetch each candidate URL until one returns valid XML content
	for _, url := range candidateURLs {
		fmt.Println("Trying to download subtitle from:", url)
	
		js := fmt.Sprintf(`await fetch(%q).then(r => r.text())`, url)
		data, err := page.Evaluate(rod.Eval(js))

		if err != nil {
			fmt.Println("❌ JS fetch failed:", err)
			continue
		}
	
		body := data.Value.Str()
		subtitleData := []byte(body)
	
		var transcript Transcript
		err = xml.Unmarshal(subtitleData, &transcript)
		if err == nil && len(transcript.Texts) > 0 {
			validURL = url
			break
		} else {
			fmt.Println("⚠️ Not valid XML transcript or empty")
		}
	}
	

	if subtitleData == nil {
		fmt.Println("⚠️ No valid subtitle XML found from captured URLs")
		return
	}

	fmt.Println("✅ Using subtitle URL:", validURL)

	os.WriteFile("subtitle.xml", subtitleData, 0644)
	fmt.Println("✅ Saved subtitle.xml")

	var transcript Transcript
	if err := xml.Unmarshal(subtitleData, &transcript); err != nil {
		panic(err)
	}

	var textBuilder strings.Builder
	for _, line := range transcript.Texts {
		textBuilder.WriteString(line.Body)
		textBuilder.WriteString(" ")
	}
	os.WriteFile("subtitle.txt", []byte(textBuilder.String()), 0644)
	fmt.Println("✅ Saved subtitle.txt")
}
