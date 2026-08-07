package relay

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const subdomainIDBytes = 5

var (
	ErrClientIDRequired    = errors.New("client ID is required")
	ErrSubdomainGeneration = errors.New("could not generate a unique subdomain")
)

type Tunnel struct {
	ID        string
	Subdomain string
	ClientID  string
	CreatedAt time.Time
}

type Registry struct {
	mu          sync.RWMutex
	bySubdomain map[string]Tunnel
	byClient    map[string]map[string]struct{}
	generateID  idGenerator
	now         func() time.Time
}

func NewRegistry() *Registry {
	return &Registry{
		bySubdomain: make(map[string]Tunnel),
		byClient:    make(map[string]map[string]struct{}),
		generateID:  func() (string, error) { return secureID(subdomainIDBytes) },
		now:         time.Now,
	}
}

func (registry *Registry) Register(clientID string) (Tunnel, error) {
	if clientID == "" {
		return Tunnel{}, ErrClientIDRequired
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	for range 10 {
		subdomain, err := registry.generateID()
		if err != nil {
			return Tunnel{}, err
		}
		if _, exists := registry.bySubdomain[subdomain]; exists {
			continue
		}

		tunnelID, err := secureID(16)
		if err != nil {
			return Tunnel{}, err
		}
		tunnel := Tunnel{ID: tunnelID, Subdomain: subdomain, ClientID: clientID, CreatedAt: registry.now().UTC()}
		registry.bySubdomain[subdomain] = tunnel
		if registry.byClient[clientID] == nil {
			registry.byClient[clientID] = make(map[string]struct{})
		}
		registry.byClient[clientID][subdomain] = struct{}{}
		return tunnel, nil
	}
	return Tunnel{}, fmt.Errorf("%w after 10 attempts", ErrSubdomainGeneration)
}

func (registry *Registry) Get(subdomain string) (Tunnel, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	tunnel, exists := registry.bySubdomain[subdomain]
	return tunnel, exists
}

func (registry *Registry) Remove(subdomain string) (Tunnel, bool) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return registry.removeLocked(subdomain)
}

func (registry *Registry) RemoveClient(clientID string) []Tunnel {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	subdomains := registry.byClient[clientID]
	removed := make([]Tunnel, 0, len(subdomains))
	for subdomain := range subdomains {
		if tunnel, exists := registry.removeLocked(subdomain); exists {
			removed = append(removed, tunnel)
		}
	}
	return removed
}

func (registry *Registry) Count() int {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return len(registry.bySubdomain)
}

func (registry *Registry) removeLocked(subdomain string) (Tunnel, bool) {
	tunnel, exists := registry.bySubdomain[subdomain]
	if !exists {
		return Tunnel{}, false
	}
	delete(registry.bySubdomain, subdomain)
	delete(registry.byClient[tunnel.ClientID], subdomain)
	if len(registry.byClient[tunnel.ClientID]) == 0 {
		delete(registry.byClient, tunnel.ClientID)
	}
	return tunnel, true
}
