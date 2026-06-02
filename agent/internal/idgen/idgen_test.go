package idgen

import (
	"os"
	"testing"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID("host1", "10.0.0.1")
	if len(id1) != 16 {
		t.Errorf("expected 16 chars, got %d", len(id1))
	}
	// Different inputs should produce different IDs deterministically
	idA := GenerateID("hostA", "10.0.0.1")
	idB := GenerateID("hostB", "10.0.0.1")
	if idA == idB {
		t.Errorf("expected different IDs for different hosts")
	}
}

func TestLoadOrCreateID(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "agent_id_*")
	f.Close()
	path := f.Name()

	id1, err := LoadOrCreateID(path, "host1", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := LoadOrCreateID(path, "host1", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("expected same ID on reload, got %s != %s", id1, id2)
	}
}
