package videostate

import (
	"testing"
	"time"
)

func TestAddAndCheck(t *testing.T) {
	p := NewProcessor()

	video := ProcessingVideo{
		VideoID: "abc123",
		Title:   "Test Video",
	}

	p.Add(video)

	if !p.Exists("abc123") {
		t.Errorf("Expected video to exist in processing list")
	}

	p.Cleanup()

	if !p.Exists("abc123") {
		t.Errorf("Expected video to remain after cleanup")
	}
}



func TestCleanupRemovesExpired(t *testing.T) {
	p := NewProcessor()

	// Simulate old video manually
	video := ProcessingVideo{
		VideoID: "old123",
		Created: time.Now().Add(-20 * time.Second),
	}

	p.mu.Lock()
	p.videos = append(p.videos, video)
	p.mu.Unlock()

	p.Cleanup()

	if p.IsBeingProcessed("old123") {
		t.Errorf("Expected expired video to be cleaned up")
	}
}
