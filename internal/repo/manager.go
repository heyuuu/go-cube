package repo

type Manager struct {
	hubs []*Hub
}

func NewManager(hubs []*Hub) *Manager {
	return &Manager{hubs: hubs}
}

func (m *Manager) Hubs() []*Hub {
	return m.hubs
}
