package videostate

import (
	"reflect"
	"sync"
	"time"
)

type Metadata struct {
	Title                 map[string]string `json:"title,omitempty"`			// multilingual
	Vid                   string `json:"videoId"`
	Lang                  string `json:"lang"`
	VideoLang         	  string `json:"video_lang,omitempty"`
	Category              string `json:"category,omitempty"`
    Summary            	  map[string]string `json:"summary,omitempty"`          // multilingual
    Answer                map[string]string `json:"answer,omitempty"`           // multilingual
    Path               	  map[string]string `json:"path,omitempty"`             // multilingual
    Status             	  map[string]string `json:"status,omitempty"`           // multilingual
	ChannelId            string `json:"channel_id,omitempty"`
	UploadDate            string `json:"video_upload_date,omitempty"`
	ChannelName			  string `json:"channel_name,omitempty"`
	ArticleUploadDateTime string `json:"article_update_datetime,omitempty"`
	Duration              int `json:"duration,omitempty"`
	LikeCount			  int `json:"like_count,omitempty"`
	DownSubDownloadCap	  string `json:"downsub_download_cap,omitempty"`
}

type VideoStatus string

type ProcessingVideo struct {
	VideoID          string
	Language         string
	Pipeline		 string
	SubtitleContent  string
	RetrySummary	 bool
	Expires          time.Time
	Status           VideoStatus
	Metadata		 Metadata
	TTLMetadata		 int
}

const (
	StatusPending              VideoStatus = "processing-pending"
	StatusMetadataProcessed    VideoStatus = "metadata-processed"
	StatusDownloadProcessed    VideoStatus = "download-processed"
	StatusDownloadAWSProcessed VideoStatus = "download-aws-processed"
	StatusSummarizeProcessed   VideoStatus = "completed"
	StatusMetadataTTlExceeded  VideoStatus = "error-metadata-ttl-exceeded" 
)


type Processor struct {
	mu     	sync.Mutex
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

func mergeNonZero(dst, src interface{}) {
    dstVal := reflect.ValueOf(dst)
    srcVal := reflect.ValueOf(src)

    if dstVal.Kind() == reflect.Ptr {
        dstVal = dstVal.Elem()
    }
    if srcVal.Kind() == reflect.Ptr {
        srcVal = srcVal.Elem()
    }

    for i := 0; i < srcVal.NumField(); i++ {
        field := srcVal.Field(i)
        dstField := dstVal.Field(i)

        if !dstField.CanSet() {
            continue
        }

        if field.Kind() == reflect.Struct && field.Type().PkgPath() == "" {
            mergeNonZero(dstField.Addr().Interface(), field.Addr().Interface())
            continue
        }

        if !isZero(field) {
            dstField.Set(field)
        }
    }
}




func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Struct:
		if t, ok := v.Interface().(time.Time); ok {
			return t.IsZero()
		}
		return false
	}
	zero := reflect.Zero(v.Type())
	return reflect.DeepEqual(v.Interface(), zero.Interface())
}

func (p *Processor) Videos() []ProcessingVideo {
    p.mu.Lock()
    defer p.mu.Unlock()

    // Cria uma cópia para evitar que o chamador modifique o slice interno
    copied := make([]ProcessingVideo, len(p.videos))
    copy(copied, p.videos)
    return copied
}



func (p *Processor) Add(newVideo ProcessingVideo) {
    p.mu.Lock()
    defer p.mu.Unlock()

	var secondsUntilExpires = 40

    timeNow := time.Now().UTC()

    for i, existing := range p.videos {
        if existing.VideoID == newVideo.VideoID && existing.Language == newVideo.Language {

            if newVideo.Status != "" {
                p.videos[i].Status = newVideo.Status
            }
            if !newVideo.Expires.IsZero() {
                p.videos[i].Expires = newVideo.Expires
            }
            // Merge Metadata recursively
            mergeNonZero(&p.videos[i].Metadata, &newVideo.Metadata)
			if (newVideo.Status != "") {
				existing.Status =  newVideo.Status
			}else{
				if (existing.Status != "") {
					newVideo.Status = existing.Status
				}
			}
			
            return
        }
    }

    newVideo.Expires = timeNow.Add(time.Duration(secondsUntilExpires) * time.Second)
    newVideo.Status = StatusPending
    p.videos = append(p.videos, newVideo)
}

func (p *Processor) SetRetrySummaryStatus(videoID string, language string, retrySummary bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			p.videos[i].RetrySummary = retrySummary
			return
		}
	}
}

func (p *Processor) GetRetrySummaryStatus(videoID string, language string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			return v.RetrySummary
		}
	}
	return false
}




func (p *Processor) DecreaseTTLMetadata(videoID string, language string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			p.videos[i].TTLMetadata = p.videos[i].TTLMetadata - 1
			return
		}
	}
	return
}

func (p *Processor) GetTTLMetadata(videoID string, language string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			return v.TTLMetadata
		}
	}
	return 0
}

func (p *Processor) SetTTLMetadata(videoID string, language string, ttl int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			p.videos[i].TTLMetadata = ttl
			return
		}
	}
	return
}

func (p *Processor) SetPipeline(videoID string, language string, pipeline string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			p.videos[i].Pipeline = pipeline
			return
		}
	}
	return
}

func (p *Processor) GetPipeline(videoID string, language string) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			return v.Pipeline
		}
	}
	return ""
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

func (p *Processor) SetStatus(videoID string, language string, videoState VideoStatus){
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, v := range p.videos {
		if v.VideoID == videoID && v.Language == language {
			p.videos[i].Status = videoState
			if p.videos[i].Metadata.Status == nil {
				p.videos[i].Metadata.Status = make(map[string]string)
			}
			p.videos[i].Metadata.Status[language] = string(videoState)
			return
		}
	}
	return 
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