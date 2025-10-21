package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"sitemap-generator/dynamo"
	sshpusher "sitemap-generator/ssh-pusher"

	xmlgen "sitemap-generator/xml-generator"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/joho/godotenv"
)





func main() {
	
	_ = godotenv.Load(".env")
	
	var dynamoDBTableName = os.Getenv("DYNAMODB_TABLE_NAME")
	if dynamoDBTableName == "" {
		log.Println("Please add DYNAMODB_TABLE_NAME to .env file")
	}

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
			log.Fatalf("Ops! failed to push sitemap: %v", err)
		}
		fmt.Printf("✅ Successfully pushed sitemap-%s.xml to server\n", lang)
	case "copy":
        destPath := os.Getenv("DESTINATION_PATH_LOCALLY")
        if destPath == "" {
            log.Fatal("DESTINATION_PATH_LOCALLY environment variable is not set")
        }

        // Ensure directory exists
        if err := os.MkdirAll(destPath, 0755); err != nil {
            log.Fatalf("Failed to create destination directory: %v", err)
        }

        // Create the destination file
        filename := fmt.Sprintf("sitemap-%s.xml", lang)
        destFile := filepath.Join(destPath, filename)
        
        // Write the sitemap content to the file
        err := os.WriteFile(destFile, []byte(sitemap), 0644)
        if err != nil {
            log.Fatalf("Failed to write sitemap file: %v", err)
        }
        
        fmt.Printf("✅ Successfully copied sitemap-%s.xml to %s\n", lang, destPath)

	default:
		log.Fatalf("Unknown action: %s (expected gen or push)", action)
	}
}
