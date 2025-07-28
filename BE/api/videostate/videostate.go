package videostate

import (
	"reflect"
	"sync"
	"time"
)

type Metadata struct {
	Title                 string `json:"title,omitempty"`
	Vid                   string `json:"videoId"`
	Lang                  string `json:"lang"`
	VideoLang         	  string `json:"video_language,omitempty"`
	Category              string `json:"category,omitempty"`
	Answer                string `json:"answer,omitempty"`
	Summary               string `json:"summary,omitempty"`
	Path                  string `json:"path,omitempty"`
	Status                string `json:"status"` // Mandatory field
	UploaderID            string `json:"uploader_id,omitempty"`
	UploadDate            string `json:"video_upload_date,omitempty"`
	ChannelID			  string `json:"channel_id,omitempty"`
	ArticleUploadDateTime string `json:"article_update_datetime,omitempty"`
	Duration              int `json:"duration,omitempty"`
	LikeCount			  int `json:"like_count,omitempty"`
	DownSubDownloadCap	  string `json:"downsub_download_cap,omitempty"`
}

type VideoStatus string

type ProcessingVideo struct {
	VideoID          string
	Language         string
	SubtitleContent  string
	Expires          time.Time
	Status           VideoStatus
	Metadata		 Metadata
}

const (
	StatusPending              VideoStatus = "processing-pending"
	StatusMetadataProcessed    VideoStatus = "metadata-processed"
	StatusDownloadProcessed    VideoStatus = "download-processed"
	StatusSummarizeProcessed   VideoStatus = "completed"
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
            // Merge Metadata recursivamente
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
			p.videos[i].Metadata.Status = string(videoState)
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