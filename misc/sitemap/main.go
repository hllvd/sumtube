package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"sitemap-generator/dynamo"
	sshpusher "sitemap-generator/ssh-pusher"

	xmlgen "sitemap-generator/xml-generator"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/joho/godotenv"
)

const dynamoDBTableName = "SummarizedSubtitles"

func main() {
	_ = godotenv.Load(".env")

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <command>, e.g., `go run main.go push-pt` or `go run main.go gen-en`")
	}

	cmd := os.Args[1] // e.g., "push-pt" or "gen-en"
	parts := strings.SplitN(cmd, "-", 2)
	if len(parts) != 2 {
		log.Fatalf("Invalid command format: %s (expected <action>-<lang>)", cmd)
	}

	action := parts[0] // "push" or "gen"
	lang := parts[1]   // "pt", "en", etc.

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal("unable to load AWS SDK config: ", err)
	}

	client := dynamo.NewDynamoClient(cfg, dynamoDBTableName)

	items, _, err := client.ListVideosByLang(context.TODO(), lang, 1000, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Generate sitemap XML
	sitemap, err := xmlgen.BuildSitemap(items, lang)
	if err != nil {
		panic(err)
	}

	switch action {
	case "gen":
		// just print sitemap to stdout
		fmt.Println(sitemap)

	case "push":
		if err := sshpusher.PushSitemap(lang, sitemap); err != nil {
			log.Fatalf("Ops! :( failed to push sitemap: %v", err)
		}
		fmt.Printf("✅ Successfully pushed sitemap-%s.xml to server\n", lang)

	default:
		log.Fatalf("Unknown action: %s (expected gen or push)", action)
	}
}
