package types

import (
	"github.com/btm-stats/protocol/bc"
)

// BlockCommitment store the TransactionsMerkleRoot && TransactionStatusHash
type BlockCommitment struct {
	// TransactionsMerkleRoot is the root hash of the Merkle binary hash tree
	// formed by the hashes of all transactions included in the block.
	TransactionsMerkleRoot bc.Hash `json:"transaction_merkle_root"`

	// TransactionStatusHash is the root hash of the Merkle binary hash tree
	// formed by the hashes of all transaction verify results
	TransactionStatusHash bc.Hash `json:"transaction_status_hash"`
}
