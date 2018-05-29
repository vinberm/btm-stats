package protocol

import (
	"sync"
	"github.com/golang/groupcache/lru"
	"github.com/btm-stats/protocol/bc/types"
	"github.com/btm-stats/protocol/bc"
	"github.com/btm-stats/errors"
	"time"
)

var (
	maxCachedErrTxs = 1000
	maxNewTxChSize  = 1000
	maxNewTxNum     = 10000

	// ErrTransactionNotExist is the pre-defined error message
	ErrTransactionNotExist = errors.New("transaction are not existed in the mempool")
	// ErrPoolIsFull indicates the pool is full
	ErrPoolIsFull = errors.New("transaction pool reach the max number")
)

// TxDesc store tx and related info for mining strategy
type TxDesc struct {
	Tx       *types.Tx
	Added    time.Time
	Height   uint64
	Weight   uint64
	Fee      uint64
	FeePerKB uint64
}

// TxPool is use for store the unconfirmed transaction
type TxPool struct {
	lastUpdated int64
	mtx         sync.RWMutex
	pool        map[bc.Hash]*TxDesc
	utxo        map[bc.Hash]bc.Hash
	errCache    *lru.Cache
	newTxCh     chan *types.Tx
}

// NewTxPool init a new TxPool
func NewTxPool() *TxPool {
	return &TxPool{
		lastUpdated: time.Now().Unix(),
		pool:        make(map[bc.Hash]*TxDesc),
		utxo:        make(map[bc.Hash]bc.Hash),
		errCache:    lru.New(maxCachedErrTxs),
		newTxCh:     make(chan *types.Tx, maxNewTxChSize),
	}
}
