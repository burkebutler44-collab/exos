package hypervisor

import "testing"

func TestFileCommandStateStoreRoundTrip(t *testing.T) {
	store := NewFileCommandStateStore(t.TempDir())

	_, ok, err := store.Get("cmd-1")
	if err != nil {
		t.Fatalf("get missing command: %v", err)
	}
	if ok {
		t.Fatal("expected missing command")
	}

	want := StoredCommandResult{
		CommandID: "cmd-1",
		Name:      "vm-1",
		Status:    "created",
		Message:   "ok",
	}
	if err := store.Put("cmd-1", want); err != nil {
		t.Fatalf("put command: %v", err)
	}

	got, ok, err := store.Get("cmd-1")
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if !ok {
		t.Fatal("expected stored command")
	}
	if got.CommandID != want.CommandID || got.Name != want.Name || got.Status != want.Status || got.Message != want.Message {
		t.Fatalf("stored result mismatch: got %+v want %+v", got, want)
	}
	if got.CompletedAt.IsZero() {
		t.Fatal("expected completed timestamp")
	}
}

func TestFileCommandStateStoreRejectsUnsafeIDs(t *testing.T) {
	store := NewFileCommandStateStore(t.TempDir())
	if err := store.Put("../cmd-1", StoredCommandResult{}); err == nil {
		t.Fatal("expected unsafe command id to be rejected")
	}
}
