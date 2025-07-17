package videostate

import (
	"sync"
	"time"
)

type Metadata struct {
	Title                 string `json:"title,omitempty"`
	Vid                   string `json:"videoId"`
	Content               string `json:"content,omitempty"`
	Lang                  string `json:"lang"`
	Category              string `json:"category,omitempty"`
	Answer                string `json:"answer,omitempty"`
	Path                  string `json:"path,omitempty"`
	Status                string `json:"status"` // Mandatory field
	UploaderID            string `json:"uploader_id,omitempty"`
	UploadDate            string `json:"video_upload_date,omitempty"`
	ArticleUploadDateTime string `json:"article_update_datetime,omitempty"`
	Duration              string `json:"duration,omitempty"`
}

type VideoStatus string

type ProcessingVideo struct {
	VideoID          string
	Language         string
	Expires          time.Time
	CapsDownloadUrl  string
	Status           VideoStatus
	Metadata		 Metadata
}

const (
	StatusPending              VideoStatus = "processing-pending"
	StatusProcessingMetadata   VideoStatus = "processing-metadata"
	StatusMetadataProcessed    VideoStatus = "metadata-processed"
	StatusProcessingDownload   VideoStatus = "processing-download"
	StatusDownloadProcessed    VideoStatus = "download-processed"
	StatusProcessingSummarize  VideoStatus = "processing-summarize"
	StatusSummarizeProcessed   VideoStatus = "summarize-processed"
)

type Processor struct {
	mu     sync.Mutex
	videos []ProcessingVideo
}

func NewProcessor() *Processor {
	return &Processor{
		videos: make([]ProcessingVideo, 0),
	}
}

func (p *Processor) Exists(videoID string, language string) bool {
	p.Cleanup()
	
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			return true
		}
	}
	return false
}


func (p *Processor) Add(processingVideo ProcessingVideo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	timeNow := time.Now().UTC()

	for i, v := range p.videos {
		if v.VideoID == processingVideo.VideoID && v.Language == processingVideo.Language {
			p.videos[i] = processingVideo
			return
		}
	}

	processingVideo.Expires = timeNow.Add(20 * time.Second)
	processingVideo.Status = StatusPending
	p.videos = append(p.videos, processingVideo)
}


func (p *Processor) GetStatus(videoID string, language string) VideoStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			return v.Status
		}
	}
	return ""
}


// Remove expired videos from processing queue
func (p *Processor) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	newList := make([]ProcessingVideo, 0)

	for _, v := range p.videos {
		if v.Expires.After(now) {
			newList = append(newList, v)
		}
	}

	p.videos = newList
}

func (p *Processor) GetVideoMeta(videoID string, language string) (*Metadata ) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			metaCopy := v.Metadata
			return &metaCopy
		}
	}
	return nil
}