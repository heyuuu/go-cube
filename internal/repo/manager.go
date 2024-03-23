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

func (m *Manager) FindHubByHost(host string) *Hub {
	for _, hub := range m.hubs {
		if hub.host == host {
			return hub
		}
	}
	return nil
}
