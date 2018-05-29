package types

import (
	"github.com/btm-stats/encoding/blockchain"
	"github.com/btm-stats/errors"
	"github.com/btm-stats/protocol/bc"
)

// TxOutput is the top level struct of tx output.
type TxOutput struct {
	AssetVersion uint64
	OutputCommitment
	// Unconsumed suffixes of the commitment and witness extensible strings.
	CommitmentSuffix []byte
}

// NewTxOutput create a new output struct
func NewTxOutput(assetID bc.AssetID, amount uint64, controlProgram []byte) *TxOutput {
	return &TxOutput{
		AssetVersion: 1,
		OutputCommitment: OutputCommitment{
			AssetAmount: bc.AssetAmount{
				AssetId: &assetID,
				Amount:  amount,
			},
			VMVersion:      1,
			ControlProgram: controlProgram,
		},
	}
}


func (to *TxOutput) readFrom(r *blockchain.Reader) (err error) {
	if to.AssetVersion, err = blockchain.ReadVarint63(r); err != nil {
		return errors.Wrap(err, "reading asset version")
	}

	if to.CommitmentSuffix, err = to.OutputCommitment.readFrom(r, to.AssetVersion); err != nil {
		return errors.Wrap(err, "reading output commitment")
	}

	// read and ignore the (empty) output witness
	_, err = blockchain.ReadVarstr31(r)
	return errors.Wrap(err, "reading output witness")
}
