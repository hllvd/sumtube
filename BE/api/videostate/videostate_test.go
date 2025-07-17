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
