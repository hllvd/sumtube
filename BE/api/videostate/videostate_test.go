package videostate

import (
	"testing"
	"time"
)

func TestAddAndCleanup(t *testing.T) {
	p := NewProcessor()

	video := ProcessingVideo{
		VideoID: "abc123",
		Language: "pt",
	}

	p.Add(video)

	if !p.Exists("abc123","pt") {
		t.Errorf("Expected video to exist in processing list")
	}

	p.Cleanup()

	if !p.Exists("abc123", "pt") {
		t.Errorf("Expected video to remain after cleanup")
	}
}

func TestGetStatus(t *testing.T) {
	p := NewProcessor()

	video := ProcessingVideo{
		VideoID:  "xyz789",
		Language: "en",
		Status:   StatusDownloadProcessed,
	}

	p.Add(video)

	status := p.GetStatus("xyz789", "en")
	if status != StatusPending { // Status gets reset to StatusPending in Add()
		t.Errorf("Expected status to be '%s', got '%s'", StatusPending, status)
	}

	// Update status directly to test another scenario
	p.mu.Lock()
	for i := range p.videos {
		if p.videos[i].VideoID == "xyz789" && p.videos[i].Language == "en" {
			p.videos[i].Status = StatusDownloadProcessed
		}
	}
	p.mu.Unlock()

	status = p.GetStatus("xyz789", "en")
	if status != StatusDownloadProcessed {
		t.Errorf("Expected updated status to be '%s', got '%s'", StatusDownloadProcessed, status)
	}

	// Test non-existing video
	status = p.GetStatus("nonexistent", "en")
	if status != "" {
		t.Errorf("Expected empty status for nonexistent video, got '%s'", status)
	}
}


func TestAddAndExists(t *testing.T) {
	p := NewProcessor()

	video := ProcessingVideo{
		VideoID:  "test123",
		Language: "en",
	}

	// Initially, it should not exist
	if p.Exists(video.VideoID, video.Language) {
		t.Errorf("Expected video to NOT exist before adding")
	}

	// Add the video
	p.Add(video)

	// Now it should exist
	if !p.Exists(video.VideoID, video.Language) {
		t.Errorf("Expected video to exist after adding")
	}
}


func TestProcessor_Add(t *testing.T) {
	p := NewProcessor()

	vid := "abc123"
	lang := "en"

	// 1. Adiciona um novo vídeo
	video1 := ProcessingVideo{
		VideoID:  vid,
		Language: lang,
		Metadata:  Metadata{
			Title: "Initial Title",
		},
	}
	p.Add(video1)

	// Acesso direto via GetVideoMeta
	meta := p.GetVideoMeta(vid, lang)
	if meta == nil {
		t.Fatal("Expected metadata, got nil")
	}
	if meta.Title != "Initial Title" {
		t.Errorf("Expected title to be 'Initial Title', got '%s'", meta.Title)
	}

	status := p.GetStatus(vid, lang)
	if status != StatusPending {
		t.Errorf("Expected status to be '%s', got '%s'", StatusPending, status)
	}

	// 2. Atualiza somente o campo `Content`
	video2 := ProcessingVideo{
		VideoID:  vid,
		Language: lang,
		Metadata:  Metadata{
			Content: "Updated content",
		},
	}
	p.Add(video2)

	meta = p.GetVideoMeta(vid, lang)
	if meta == nil {
		t.Fatal("Expected metadata after update, got nil")
	}

	if meta.Title != "Initial Title" {
		t.Errorf("Expected title to remain 'Initial Title', got '%s'", meta.Title)
	}

	if meta.Content != "Updated content" {
		t.Errorf("Expected content to be 'Updated content', got '%s'", meta.Content)
	}
}

func TestProcessor_Add_MultipleVideos(t *testing.T) {
	p := NewProcessor()

	videos := []ProcessingVideo{
		{VideoID: "vid1", Language: "en", Metadata: Metadata{Title: "Title 1"}},
		{VideoID: "vid2", Language: "pt", Metadata: Metadata{Title: "Título 2"}},
		{VideoID: "vid3", Language: "en", Metadata: Metadata{Title: "Title 3"}},
	}

	// Adiciona vários vídeos
	for _, v := range videos {
		p.Add(v)
	}

	if len(p.Videos()) != 3 {
		t.Errorf("Expected 3 videos, got %d", len(p.Videos()))
	}

	// Confere título do segundo vídeo
	meta := p.GetVideoMeta("vid2", "pt")
	if meta == nil || meta.Title != "Título 2" {
		t.Errorf("Expected 'Título 2', got '%v'", meta)
	}
}

func TestProcessor_Add_UpdateMultipleFields(t *testing.T) {
	p := NewProcessor()

	vid := "vidX"
	lang := "en"

	video := ProcessingVideo{
		VideoID:  vid,
		Language: lang,
		Metadata: Metadata{Title: "Original Title", Content: "Original content"},
		Status:   StatusPending,
	}
	p.Add(video)

	// Atualiza Title, Content, Status, CapsDownloadUrl e Expires
	newExpire := time.Now().Add(30 * time.Second)
	update := ProcessingVideo{
		VideoID:         vid,
		Language:        lang,
		Metadata:        Metadata{Title: "Updated Title", Content: "Updated content"},
		Status:          StatusProcessingDownload,
		CapsDownloadUrl: "http://example.com/caps",
		Expires:         newExpire,
	}
	p.Add(update)

	meta := p.GetVideoMeta(vid, lang)
	if meta.Title != "Updated Title" || meta.Content != "Updated content" {
		t.Errorf("Expected metadata updated, got Title='%s' Content='%s'", meta.Title, meta.Content)
	}

	status := p.GetStatus(vid, lang)
	if status != StatusProcessingDownload {
		t.Errorf("Expected status '%s', got '%s'", StatusProcessingDownload, status)
	}

	videos := p.Videos()
	if len(videos) != 1 {
		t.Fatalf("Expected 1 video, got %d", len(videos))
	}

	if videos[0].CapsDownloadUrl != "http://example.com/caps" {
		t.Errorf("Expected CapsDownloadUrl updated, got '%s'", videos[0].CapsDownloadUrl)
	}

	if !videos[0].Expires.Equal(newExpire) {
		t.Errorf("Expected Expires to be updated, got %v", videos[0].Expires)
	}
}





func TestCleanupRemovesExpired(t *testing.T) {
	p := NewProcessor()

	// Simulate old video manually
	video := ProcessingVideo{
		VideoID: "old123",
		Language: "pt",
		Expires: time.Now().Add(-30 * time.Second),
	}

	p.mu.Lock()
	p.videos = append(p.videos, video)
	p.mu.Unlock()

	p.Cleanup()

	if p.Exists("old123","pt") {
		t.Errorf("Expected expired video to be cleaned up")
	}
}
