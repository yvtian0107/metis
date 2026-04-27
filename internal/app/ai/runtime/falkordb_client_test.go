package runtime

import "testing"

func TestFalkorDBClientShutdownNilSafe(t *testing.T) {
	var client *FalkorDBClient
	if err := client.Shutdown(); err != nil {
		t.Fatalf("nil shutdown returned error: %v", err)
	}

	client = &FalkorDBClient{}
	if err := client.Shutdown(); err != nil {
		t.Fatalf("empty shutdown returned error: %v", err)
	}
}
