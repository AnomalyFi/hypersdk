package anchor

import (
	"sync"

	"github.com/AnomalyFi/hypersdk/actions"
	"github.com/ava-labs/avalanchego/ids"
)

type AnchorRegistry struct {
	anchors  map[ids.ID]*actions.AnchorInfo
	anchorsL sync.Mutex
}

func NewAnchorRegistry() *AnchorRegistry {
	return &AnchorRegistry{
		anchors: make(map[ids.ID]*actions.AnchorInfo),
	}
}

func (r *AnchorRegistry) Update(info []*actions.AnchorInfo) {
	r.anchorsL.Lock()
	defer r.anchorsL.Unlock()

	r.anchors = make(map[ids.ID]*actions.AnchorInfo, len(info))
	for _, a := range info {
		id := a.ID()
		r.anchors[id] = a
	}
}

func (r *AnchorRegistry) Len() int {
	return len(r.anchors)
}
