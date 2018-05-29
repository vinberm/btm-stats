package leveldb

import (
	"sync"

	"github.com/golang/groupcache/lru"
	"github.com/golang/groupcache/singleflight"

	"github.com/btm-stats/protocol/bc"
	"github.com/btm-stats/protocol/bc/types"
)

const maxCachedBlocks = 30

func newBlockCache(fillFn func(hash *bc.Hash) *types.Block) blockCache {
	return blockCache{
		lru:    lru.New(maxCachedBlocks),
		fillFn: fillFn,
	}
}

type blockCache struct {
	mu     sync.Mutex
	lru    *lru.Cache
	fillFn func(hash *bc.Hash) *types.Block
	single singleflight.Group
}
