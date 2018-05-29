package types

import (
	"github.com/btm-stats/protocol/bc"
	"io"
	"fmt"
	"github.com/btm-stats/encoding/blockchain"
)

// BlockHeader defines information about a block and is used in the Bytom
type BlockHeader struct {
	Version           uint64  // The version of the block.
	Height            uint64  // The height of the block.
	PreviousBlockHash bc.Hash // The hash of the previous block.
	Timestamp         uint64  // The time of the block in seconds.
	Nonce             uint64  // Nonce used to generate the block.
	Bits              uint64  // Difficulty target for the block.
	BlockCommitment
}

// Hash returns complete hash of the block header.
func (bh *BlockHeader) Hash() bc.Hash {
	h, _ := mapBlockHeader(bh)
	return h
}

func (bh *BlockHeader) readFrom(r *blockchain.Reader) (serflag uint8, err error) {
	var serflags [1]byte
	io.ReadFull(r, serflags[:])
	serflag = serflags[0]
	switch serflag {
	case SerBlockHeader, SerBlockFull:
	default:
		return 0, fmt.Errorf("unsupported serialization flags 0x%x", serflags)
	}

	if bh.Version, err = blockchain.ReadVarint63(r); err != nil {
		return 0, err
	}
	if bh.Height, err = blockchain.ReadVarint63(r); err != nil {
		return 0, err
	}
	if _, err = bh.PreviousBlockHash.ReadFrom(r); err != nil {
		return 0, err
	}
	if bh.Timestamp, err = blockchain.ReadVarint63(r); err != nil {
		return 0, err
	}
	if _, err = blockchain.ReadExtensibleString(r, bh.BlockCommitment.readFrom); err != nil {
		return 0, err
	}
	if bh.Nonce, err = blockchain.ReadVarint63(r); err != nil {
		return 0, err
	}
	if bh.Bits, err = blockchain.ReadVarint63(r); err != nil {
		return 0, err
	}
	return
}
