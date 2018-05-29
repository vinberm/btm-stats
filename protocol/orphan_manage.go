package protocol

import (
	"sync"
	"github.com/btm-stats/protocol/bc"
	"github.com/btm-stats/protocol/bc/types"
)


// OrphanManage is use to handle all the orphan block
type OrphanManage struct {
	//TODO: add orphan cached block limit
	orphan      map[bc.Hash]*types.Block
	prevOrphans map[bc.Hash][]*bc.Hash
	mtx         sync.RWMutex
}

// NewOrphanManage return a new orphan block
func NewOrphanManage() *OrphanManage {
	return &OrphanManage{
		orphan:      make(map[bc.Hash]*types.Block),
		prevOrphans: make(map[bc.Hash][]*bc.Hash),
	}
}
