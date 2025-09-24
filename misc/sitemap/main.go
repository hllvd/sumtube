package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"sitemap-generator/dynamo"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/joho/godotenv"
)

const dynamoDBTableName = "SummarizedSubtitles"

func main() {
	// Load .env file
	_ = godotenv.Load(".env")

	fmt.Println("Region:", os.Getenv("AWS_DEFAULT_REGION"))

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal("unable to load AWS SDK config: ", err)
	}

	client := dynamo.NewDynamoClient(cfg, dynamoDBTableName)

	// first page: 100 items
	items, lastKey, err := client.ListVideosByLang(context.TODO(), "en", 100, nil)
	if err != nil {
		log.Fatal(err)
	}

	for _, item := range items {
		fmt.Printf("VideoID: %s | Title: %+v | Category: %s\n | Path : %s \n", item.Vid, item.Title, item.Category, item.Path)
		// print items.LanguagesFound
		for lang, _ := range item.LanguagesFound {
			fmt.Printf("Language: %s\n", lang)
		}
		fmt.Printf("====================================")

	}

	fmt.Println("Got", len(items), "videos")

	// if more pages exist, fetch next 100
	if len(lastKey) > 0 {
		fmt.Println("Fetching next page...")
		nextItems, _, err := client.ListVideosByLang(context.TODO(), "pt", 100, lastKey)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Next page videos:", len(nextItems))
	}
}
