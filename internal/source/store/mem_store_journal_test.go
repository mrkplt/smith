package store

import (
	"context"
	"testing"
	"time"

	"smith/internal/source/model"
)

func TestMemStoreAppendJournalAssignsMonotonicSequence(t *testing.T) {
	ms := NewMemStore()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := ms.AppendJournal(ctx, model.JournalEntry{
			LoopID:  "loop-journal-order",
			Message: "entry",
		}); err != nil {
			t.Fatalf("append journal entry %d: %v", i+1, err)
		}
	}

	entries, err := ms.ListJournal(ctx, "loop-journal-order", 0)
	if err != nil {
		t.Fatalf("list journal: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	for i, entry := range entries {
		want := int64(i + 1)
		if entry.Sequence != want {
			t.Fatalf("entry %d sequence=%d want=%d", i, entry.Sequence, want)
		}
	}
}

func TestMemStoreListJournalSinceWithRevisionFiltersBySequence(t *testing.T) {
	ms := NewMemStore()
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		if err := ms.AppendJournal(ctx, model.JournalEntry{
			LoopID:  "loop-journal-since",
			Message: "entry",
		}); err != nil {
			t.Fatalf("append journal entry %d: %v", i+1, err)
		}
	}

	entries, _, err := ms.ListJournalSinceWithRevision(ctx, "loop-journal-since", 2)
	if err != nil {
		t.Fatalf("list journal since: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after since=2, got %d", len(entries))
	}
	if entries[0].Sequence != 3 || entries[1].Sequence != 4 {
		t.Fatalf("unexpected sequences after filter: %#v", entries)
	}
}

func TestMemStoreWatchJournalStreamsNewEntries(t *testing.T) {
	ms := NewMemStore()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	watch := ms.WatchJournal(ctx, "loop-journal-watch")
	if err := ms.AppendJournal(ctx, model.JournalEntry{
		LoopID:  "loop-journal-watch",
		Message: "watch-me",
	}); err != nil {
		t.Fatalf("append journal: %v", err)
	}

	select {
	case entry := <-watch:
		if entry.Message != "watch-me" {
			t.Fatalf("unexpected watched message: %q", entry.Message)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for watched journal entry")
	}
}
