package protocol

import (
	"sync"
	"github.com/btm-stats/protocol/state"
	"github.com/btm-stats/errors"
	"github.com/btm-stats/protocol/bc"
	"github.com/btm-stats/protocol/bc/types"

	"github.com/btm-stats/config"
)

const maxProcessBlockChSize = 1024

// ErrTheDistantFuture is returned when waiting for a blockheight too far in
// excess of the tip of the blockchain.
var ErrTheDistantFuture = errors.New("block height too far in future")

// Chain provides functions for working with the Bytom block chain.
type Chain struct {
	index          *state.BlockIndex
	orphanManage   *OrphanManage
	txPool         *TxPool
	store          Store
	processBlockCh chan *processBlockMsg

	cond     sync.Cond
	bestNode *state.BlockNode
}

// NewChain returns a new Chain using store as the underlying storage.
func NewChain(store Store, txPool *TxPool) (*Chain, error) {
	c := &Chain{
		orphanManage:   NewOrphanManage(),
		txPool:         txPool,
		store:          store,
		processBlockCh: make(chan *processBlockMsg, maxProcessBlockChSize),
	}
	c.cond.L = new(sync.Mutex)

	storeStatus := store.GetStoreStatus()
	if storeStatus == nil {
		if err := c.initChainStatus(); err != nil {
			return nil, err
		}
		storeStatus = store.GetStoreStatus()
	}

	var err error
	if c.index, err = store.LoadBlockIndex(); err != nil {
		return nil, err
	}

	c.bestNode = c.index.GetNode(storeStatus.Hash)
	c.index.SetMainChain(c.bestNode)
	go c.blockProcesser()
	return c, nil
}

func (c *Chain) initChainStatus() error {
	genesisBlock := config.GenesisBlock()
	txStatus := bc.NewTransactionStatus()
	for i := range genesisBlock.Transactions {
		txStatus.SetStatus(i, false)
	}

	if err := c.store.SaveBlock(genesisBlock, txStatus); err != nil {
		return err
	}

	utxoView := state.NewUtxoViewpoint()
	bcBlock := types.MapBlock(genesisBlock)
	if err := utxoView.ApplyBlock(bcBlock, txStatus); err != nil {
		return err
	}

	node, err := state.NewBlockNode(&genesisBlock.BlockHeader, nil)
	if err != nil {
		return err
	}
	return c.store.SaveChainStatus(node, utxoView)
}
