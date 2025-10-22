package model

type ApiResponseSummary struct {
    Title   string `json:"title,omitempty"`
    Vid     string `json:"videoId"`
    Content string `json:"content,omitempty"`
    Lang    string `json:"lang"`
	Category string `json:"category,omitempty"`
    Answer  string `json:"answer,omitempty"`
    Path    string `json:"path,omitempty"`
    Status  string `json:"status"` // Mandatory field
	ChannelId string `json:"channel_id,omitempty"`
	UploadDate string `json:"video_upload_date,omitempty"`
	ArticleUploadDateTime string `json:"article_update_datetime,omitempty"`
	Duration string `json:"duration,omitempty"`
}

type DynamoResponseSummary struct {
    Title   string `json:"title,omitempty"`
    Vid     string `json:"videoId"`
    Content string `json:"content,omitempty"`
    Lang    string `json:"lang"`
	Category string `json:"category,omitempty"`
    Answer  string `json:"answer,omitempty"`
    Path    string `json:"path,omitempty"`
    Status  string `json:"status"` // Mandatory field
	ChannelId string `json:"channel_id,omitempty"`
	UploadDate string `json:"video_upload_date,omitempty"`
	ArticleUploadDateTime string `json:"article_update_datetime,omitempty"`
	Duration string `json:"duration,omitempty"`
}