package repo

type Repo interface {
	Url() string
}

type Hub struct {
	name string
	host string
}

func NewHub(name string, host string) *Hub {
	return &Hub{name: name, host: host}
}

func (h *Hub) Name() string {
	return h.name
}

func (h *Hub) Host() string {
	return h.host
}
