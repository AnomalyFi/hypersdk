package arcadia

import (
	"sync"

	"github.com/AnomalyFi/hypersdk/actions"
	"github.com/ava-labs/avalanchego/ids"
)

type RollupRegistry struct {
	rollups  map[ids.ID]*actions.RollupInfo
	rollupsL sync.Mutex
}

func NewRollupRegistry() *RollupRegistry {
	return &RollupRegistry{
		rollups: make(map[ids.ID]*actions.RollupInfo),
	}
}

func (r *RollupRegistry) Update(info []*actions.RollupInfo) {
	r.rollupsL.Lock()
	defer r.rollupsL.Unlock()

	r.rollups = make(map[ids.ID]*actions.RollupInfo, len(info))
	for _, a := range info {
		id := a.ID()
		r.rollups[id] = a
	}
}

func (r *RollupRegistry) Len() int {
	return len(r.rollups)
}
