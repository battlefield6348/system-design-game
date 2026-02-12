package persistence

import (
	"fmt"
	"sync"
	"system-design-game/internal/domain/design"
)

// InMemDesignRepository 記憶體實作的 Design Repository (開發初期使用)
type InMemDesignRepository struct {
	mu      sync.RWMutex
	designs map[string]*design.Design
}

func NewInMemDesignRepository() *InMemDesignRepository {
	return &InMemDesignRepository{
		designs: make(map[string]*design.Design),
	}
}

func (r *InMemDesignRepository) Save(d *design.Design) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.designs[d.ID] = d
	return nil
}

func (r *InMemDesignRepository) GetByID(id string) (*design.Design, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.designs[id]
	if !ok {
		return nil, fmt.Errorf("design not found: %s", id)
	}
	return d, nil
}

func (r *InMemDesignRepository) ListByPlayerID(playerID string) ([]*design.Design, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*design.Design
	for _, d := range r.designs {
		if d.PlayerID == playerID {
			result = append(result, d)
		}
	}
	return result, nil
}
