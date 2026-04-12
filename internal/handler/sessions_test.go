package handler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSummarizeSessionTranscriptKeepsRecentPreviewWindow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	transcript := filepath.Join(dir, "sample.jsonl")
	content := "" +
		"{\"type\":\"message\",\"id\":\"m1\",\"timestamp\":\"2026-03-29T01:00:00Z\",\"message\":{\"role\":\"user\",\"content\":\"u1\"}}\n" +
		"{\"type\":\"assistant\",\"id\":\"a1\",\"timestamp\":\"2026-03-29T01:00:01Z\",\"message\":{\"content\":\"a1\"}}\n" +
		"{\"type\":\"message\",\"id\":\"m2\",\"timestamp\":\"2026-03-29T01:00:02Z\",\"message\":{\"role\":\"user\",\"content\":\"u2\"}}\n" +
		"{\"type\":\"assistant\",\"id\":\"a2\",\"timestamp\":\"2026-03-29T01:00:03Z\",\"message\":{\"content\":\"a2\"}}\n" +
		"{\"type\":\"message\",\"id\":\"m3\",\"timestamp\":\"2026-03-29T01:00:04Z\",\"message\":{\"role\":\"user\",\"content\":\"u3\"}}\n"
	if err := os.WriteFile(transcript, []byte(content), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	count, recent := summarizeSessionTranscript(transcript, 4)
	if count != 5 {
		t.Fatalf("expected 5 messages, got %d", count)
	}
	if len(recent) != 4 {
		t.Fatalf("expected 4 preview messages, got %d", len(recent))
	}
	if got := recent[0]["content"]; got != "a1" {
		t.Fatalf("expected preview to keep the most recent 4 messages, first content=%v", got)
	}
	if got := recent[3]["content"]; got != "u3" {
		t.Fatalf("expected last preview item to be u3, got %v", got)
	}
}
