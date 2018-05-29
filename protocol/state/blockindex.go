package state

import (
	"sync"
	"github.com/btm-stats/protocol/bc"
	"math/big"
)

// approxNodesPerDay is an approximation of the number of new blocks there are
// in a week on average.
const approxNodesPerDay = 24 * 24

// BlockNode represents a block within the block chain and is primarily used to
// aid in selecting the best chain to be the main chain.
type BlockNode struct {
	Parent  *BlockNode // parent is the parent block for this node.
	Hash    bc.Hash    // hash of the block.
	Seed    *bc.Hash   // seed hash of the block
	WorkSum *big.Int   // total amount of work in the chain up to

	Version                uint64
	Height                 uint64
	Timestamp              uint64
	Nonce                  uint64
	Bits                   uint64
	TransactionsMerkleRoot bc.Hash
	TransactionStatusHash  bc.Hash
}

// BlockIndex is the struct for help chain trace block chain as tree
type BlockIndex struct {
	sync.RWMutex

	index     map[bc.Hash]*BlockNode
	mainChain []*BlockNode
}

// GetNode will search node from the index map
func (bi *BlockIndex) GetNode(hash *bc.Hash) *BlockNode {
	bi.RLock()
	defer bi.RUnlock()
	return bi.index[*hash]
}

// SetMainChain will set the the mainChain array
func (bi *BlockIndex) SetMainChain(node *BlockNode) {
	bi.Lock()
	defer bi.Unlock()

	needed := node.Height + 1
	if uint64(cap(bi.mainChain)) < needed {
		nodes := make([]*BlockNode, needed, needed+approxNodesPerDay)
		copy(nodes, bi.mainChain)
		bi.mainChain = nodes
	} else {
		i := uint64(len(bi.mainChain))
		bi.mainChain = bi.mainChain[0:needed]
		for ; i < needed; i++ {
			bi.mainChain[i] = nil
		}
	}

	for node != nil && bi.mainChain[node.Height] != node {
		bi.mainChain[node.Height] = node
		node = node.Parent
	}
}
