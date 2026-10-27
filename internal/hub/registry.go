package hub

import (
	"slices"
	"strings"
	"sync"
	"time"
)

// AgentInfo is the hub's view of one agent. It is the domain type; the
// protobuf Agent message is only the wire format and is built from this.
type AgentInfo struct {
	ID           string
	Hostname     string
	OS           string
	Arch         string
	AgentVersion string
	IPs          []string
	RegisteredAt time.Time
	LastSeen     time.Time
}

// Registry is the hub's in-memory list of known agents. It is deliberately
// not persisted: when the hub restarts, agents notice (Heartbeat returns
// NOT_FOUND) and register again.
type Registry struct {
	mu     sync.Mutex
	agents map[string]AgentInfo
	now    func() time.Time
}

func NewRegistry() *Registry {
	return &Registry{agents: map[string]AgentInfo{}, now: time.Now}
}

// Upsert records a (re-)registration. RegisteredAt is kept from the first
// registration so a re-register after a hub hiccup doesn't reset it.
func (r *Registry) Upsert(info AgentInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	info.RegisteredAt = now
	if prev, ok := r.agents[info.ID]; ok {
		info.RegisteredAt = prev.RegisteredAt
	}
	info.LastSeen = now
	r.agents[info.ID] = info
}

// Touch marks an agent as seen now. It reports false for unknown agents.
func (r *Registry) Touch(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.agents[id]
	if !ok {
		return false
	}
	info.LastSeen = r.now()
	r.agents[id] = info
	return true
}

// List returns all agents sorted by hostname.
func (r *Registry) List() []AgentInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]AgentInfo, 0, len(r.agents))
	for _, info := range r.agents {
		out = append(out, info)
	}
	slices.SortFunc(out, func(a, b AgentInfo) int {
		return strings.Compare(a.Hostname, b.Hostname)
	})
	return out
}

// Online reports whether an agent was seen within the given window.
func (r *Registry) Online(info AgentInfo, window time.Duration) bool {
	return r.now().Sub(info.LastSeen) <= window
}
