package protocol

import (
	log "github.com/sirupsen/logrus"

	"github.com/btm-stats/protocol/bc/types"
)

type processBlockMsg struct {
	block *types.Block
	reply chan processBlockResponse
}

type processBlockResponse struct {
	isOrphan bool
	err      error
}

func (c *Chain) blockProcesser() {
	for msg := range c.processBlockCh {
		isOrphan, err := c.processBlock(msg.block)
		msg.reply <- processBlockResponse{isOrphan: isOrphan, err: err}
	}
}

// ProcessBlock is the entry for handle block insert
func (c *Chain) processBlock(block *types.Block) (bool, error) {
	blockHash := block.Hash()
	if c.BlockExist(&blockHash) {
		log.WithFields(log.Fields{"hash": blockHash.String(), "height": block.Height}).Info("block has been processed")
		return c.orphanManage.BlockExist(&blockHash), nil
	}

	if parent := c.index.GetNode(&block.PreviousBlockHash); parent == nil {
		c.orphanManage.Add(block)
		return true, nil
	}

	if err := c.saveBlock(block); err != nil {
		return false, err
	}

	bestBlock := c.saveSubBlock(block)
	bestBlockHash := bestBlock.Hash()
	bestNode := c.index.GetNode(&bestBlockHash)

	if bestNode.Parent == c.bestNode {
		log.Debug("append block to the end of mainchain")
		return false, c.connectBlock(bestBlock)
	}

	if bestNode.Height > c.bestNode.Height && bestNode.WorkSum.Cmp(c.bestNode.WorkSum) >= 0 {
		log.Debug("start to reorganize chain")
		return false, c.reorganizeChain(bestNode)
	}
	return false, nil
}

// GetBlockByHeight return a block by given height
func (c *Chain) GetBlockByHeight(height uint64) (*types.Block, error) {
	node := c.index.NodeByHeight(height)
	if node == nil {
		return nil, errors.New("can't find block in given height")
	}
	return c.store.GetBlock(&node.Hash)
}
