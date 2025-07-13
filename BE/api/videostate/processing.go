package videostate

import (
	"sync"
	"time"
)

type ProcessingVideo struct {
	VideoID          string
	Created          time.Time
	Title            string
	Duration         int
	Path             string
	VideoLang        string
	Lang             string
	CapsDownloadUrl  string
	Status           VideoStatus
}

type VideoStatus string

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

func (p *Processor) Exists(videoID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, v := range p.videos {
		if v.VideoID == videoID {
			return true
		}
	}
	return false
}


func (p *Processor) Add(processingVideo ProcessingVideo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	created := time.Now().UTC()

	for i, v := range p.videos {
		if v.VideoID == processingVideo.VideoID {
			processingVideo.Created = created
			p.videos[i] = processingVideo
			return
		}
	}

	processingVideo.Created = created
	processingVideo.Status = StatusPending
	p.videos = append(p.videos, processingVideo)
}

func (p *Processor) IsBeingProcessed(videoID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	seconds := 10
	now := time.Now()

	for _, v := range p.videos {
		if v.VideoID == videoID {
			if int(now.Sub(v.Created).Seconds()) > seconds  {
				return true
			}
		}
	}
	return false
}

func (p *Processor) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	newList := make([]ProcessingVideo, 0)

	for _, v := range p.videos {
		if now.Sub(v.Created) <= 20*time.Second {
			newList = append(newList, v)
		}
	}

	p.videos = newList
}
