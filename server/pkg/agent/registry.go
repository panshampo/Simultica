package agent

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// BackendFactory constructs a backend for a registered agent type.
type BackendFactory func(Config) (Backend, error)

var (
	registryMu       sync.RWMutex
	backendFactories = make(map[string]BackendFactory)
	launchHeaderExt  = make(map[string]string)
)

// RegisterBackend adds a package-local backend extension. It is intended for
// init-time registration by Simultica-owned backends so upstream factory code
// can stay focused on built-in Multica backends.
func RegisterBackend(agentType string, factory BackendFactory) {
	if agentType == "" {
		panic("agent: RegisterBackend called with empty agent type")
	}
	if factory == nil {
		panic(fmt.Sprintf("agent: RegisterBackend(%q) called with nil factory", agentType))
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := backendFactories[agentType]; exists {
		panic(fmt.Sprintf("agent: backend %q already registered", agentType))
	}
	backendFactories[agentType] = factory
}

// RegisterLaunchHeader adds the user-visible launch skeleton for a registered
// backend extension.
func RegisterLaunchHeader(agentType, header string) {
	if agentType == "" {
		panic("agent: RegisterLaunchHeader called with empty agent type")
	}
	if header == "" {
		panic(fmt.Sprintf("agent: RegisterLaunchHeader(%q) called with empty header", agentType))
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	launchHeaderExt[agentType] = header
}

func newRegisteredBackend(agentType string, cfg Config) (Backend, bool, error) {
	registryMu.RLock()
	factory, ok := backendFactories[agentType]
	registryMu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	backend, err := factory(cfg)
	return backend, true, err
}

func registeredLaunchHeader(agentType string) (string, bool) {
	registryMu.RLock()
	header, ok := launchHeaderExt[agentType]
	registryMu.RUnlock()
	return header, ok
}

func supportedAgentTypesString() string {
	types := append([]string(nil), builtInAgentTypes...)

	registryMu.RLock()
	for agentType := range backendFactories {
		types = append(types, agentType)
	}
	registryMu.RUnlock()

	sort.Strings(types)
	return strings.Join(types, ", ")
}
