package anchor

import (
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/actions"
)

type AnchorRegistry struct {
	anchors  map[ids.ID]*actions.AnchorInfo
	anchorsL sync.Mutex
	vm       VM
}

func NewAnchorRegistry(vm VM) *AnchorRegistry {
	return &AnchorRegistry{
		vm:      vm,
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
