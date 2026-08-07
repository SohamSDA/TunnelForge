package relay

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRegistryLifecycle(t *testing.T) {
	registry := NewRegistry()
	registry.generateID = func() (string, error) { return "abc123", nil }
	registry.now = func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.FixedZone("test", 3600)) }

	tunnel, err := registry.Register("client-1")
	if err != nil {
		t.Fatalf("Register() error: %v", err)
	}
	if tunnel.Subdomain != "abc123" || tunnel.ClientID != "client-1" || tunnel.ID == "" {
		t.Fatalf("Register() = %#v", tunnel)
	}
	if tunnel.CreatedAt.Location() != time.UTC {
		t.Fatalf("CreatedAt location = %v, want UTC", tunnel.CreatedAt.Location())
	}
	if got, exists := registry.Get("abc123"); !exists || got != tunnel {
		t.Fatalf("Get() = %#v, %v", got, exists)
	}
	if removed, exists := registry.Remove("abc123"); !exists || removed != tunnel {
		t.Fatalf("Remove() = %#v, %v", removed, exists)
	}
	if registry.Count() != 0 {
		t.Fatalf("Count() = %d, want 0", registry.Count())
	}
}

func TestRegistryRejectsMissingClient(t *testing.T) {
	if _, err := NewRegistry().Register(""); !errors.Is(err, ErrClientIDRequired) {
		t.Fatalf("Register() error = %v, want ErrClientIDRequired", err)
	}
}

func TestRegistryHandlesCollisions(t *testing.T) {
	registry := NewRegistry()
	ids := []string{"same", "same", "different"}
	registry.generateID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	if _, err := registry.Register("client-1"); err != nil {
		t.Fatal(err)
	}
	second, err := registry.Register("client-2")
	if err != nil {
		t.Fatal(err)
	}
	if second.Subdomain != "different" {
		t.Fatalf("subdomain = %q, want different", second.Subdomain)
	}
}

func TestRegistryRemoveClient(t *testing.T) {
	registry := NewRegistry()
	sequence := 0
	registry.generateID = func() (string, error) {
		sequence++
		return fmt.Sprintf("sub-%d", sequence), nil
	}
	for range 3 {
		if _, err := registry.Register("client-1"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := registry.Register("client-2"); err != nil {
		t.Fatal(err)
	}
	if removed := registry.RemoveClient("client-1"); len(removed) != 3 {
		t.Fatalf("RemoveClient() removed %d tunnels, want 3", len(removed))
	}
	if registry.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", registry.Count())
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	registry := NewRegistry()
	const workers = 100
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)
	for index := range workers {
		go func() {
			defer waitGroup.Done()
			tunnel, err := registry.Register(fmt.Sprintf("client-%d", index%10))
			if err != nil {
				t.Errorf("Register() error: %v", err)
				return
			}
			if _, exists := registry.Get(tunnel.Subdomain); !exists {
				t.Errorf("Get(%q) did not find tunnel", tunnel.Subdomain)
			}
		}()
	}
	waitGroup.Wait()
	if registry.Count() != workers {
		t.Fatalf("Count() = %d, want %d", registry.Count(), workers)
	}
}
