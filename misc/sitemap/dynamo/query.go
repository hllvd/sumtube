package dynamo

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Metadata struct {
	Title                 map[string]string `json:"title,omitempty"`
	Vid                   string            `json:"videoId"`
	Lang                  string            `json:"lang"`
	VideoLang             string            `json:"video_lang,omitempty"`
	Category              string            `json:"category,omitempty"`
	Path                  map[string]string `json:"path,omitempty"`
	Status                map[string]string `json:"status,omitempty"`
	UploadDate            string            `json:"video_upload_date,omitempty"`
	ChannelName           string            `json:"channel_name,omitempty"`
	ArticleUploadDateTime string            `json:"article_update_datetime,omitempty"`
	LanguagesFound		  map[string]bool   `json:"languages_found,omitempty"`
}

var allowedLanguages = map[string]bool{
	"en": false,
	"pt": false,
	"es": false,
	"it": false,
	"fr": false,
	"de": false,
	}

// Client wrapper
type DynamoClient struct {
	DB        *dynamodb.Client
	TableName string
}

// Factory
func NewDynamoClient(cfg aws.Config, tableName string) *DynamoClient {
	return &DynamoClient{
		DB:        dynamodb.NewFromConfig(cfg),
		TableName: tableName,
	}
}

// Paginated query by language
func (c *DynamoClient) ListVideosByLang(
    ctx context.Context,
    lang string,
    limit int32,
    startKey map[string]types.AttributeValue,
) ([]Metadata, map[string]types.AttributeValue, error) {
    input := &dynamodb.QueryInput{
        TableName:              aws.String(c.TableName),
        IndexName:              aws.String("GSI1"), // ✅ use the right index
        KeyConditionExpression: aws.String("GSI1PK = :pk"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("VIDS#")},
        },
        Limit:             aws.Int32(limit),
        ExclusiveStartKey: startKey,
        ScanIndexForward:  aws.Bool(false), // ✅ descending order (latest → oldest)
    }

    out, err := c.DB.Query(ctx, input)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to query DynamoDB: %w", err)
    }

    var videos []Metadata
    if err := attributevalue.UnmarshalListOfMaps(out.Items, &videos); err != nil {
        log.Printf("failed to unmarshal items: %v", err)
        return nil, out.LastEvaluatedKey, err
    }

    // Normalize maps and check for completed status
    for i := range videos {
        if videos[i].Title == nil {
            videos[i].Title = make(map[string]string)
        }
        if videos[i].Path == nil {
            videos[i].Path = make(map[string]string)
        }
        if videos[i].Status == nil {
            videos[i].Status = make(map[string]string)
        }
        if videos[i].LanguagesFound == nil {
            videos[i].LanguagesFound = make(map[string]bool)
        }

        // Mark which langs are completed
        for currentLang := range allowedLanguages {
            if status, exists := videos[i].Status[currentLang]; exists && status == "completed" {
                videos[i].LanguagesFound[currentLang] = true
            }
        }
    }

    return videos, out.LastEvaluatedKey, nil
}


