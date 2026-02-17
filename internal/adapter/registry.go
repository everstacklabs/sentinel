package adapter

import (
	"fmt"
	"sync"
)

var (
	mu       sync.RWMutex
	adapters = make(map[string]Adapter)
)

// Register adds an adapter to the global registry.
func Register(a Adapter) {
	mu.Lock()
	defer mu.Unlock()
	adapters[a.Name()] = a
}

// Get returns an adapter by provider name.
func Get(name string) (Adapter, error) {
	mu.RLock()
	defer mu.RUnlock()
	a, ok := adapters[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return a, nil
}

// List returns all registered adapter names.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(adapters))
	for name := range adapters {
		names = append(names, name)
	}
	return names
}
