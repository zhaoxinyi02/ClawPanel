package handler

import "testing"

func TestDedupeOpenClawTasksByID(t *testing.T) {
	t.Parallel()

	in := []openClawTaskRecord{
		{TaskID: "task-1", Status: "running", CreatedAt: 300},
		{TaskID: "task-1", Status: "failed", CreatedAt: 200},
		{TaskID: "task-2", Status: "queued", CreatedAt: 100},
	}

	got := dedupeOpenClawTasksByID(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 records after dedupe, got %d", len(got))
	}
	if got[0].TaskID != "task-1" || got[0].Status != "running" {
		t.Fatalf("expected first/latest task-1 to be kept, got %#v", got[0])
	}
	if got[1].TaskID != "task-2" {
		t.Fatalf("expected task-2 to remain, got %#v", got[1])
	}
}
