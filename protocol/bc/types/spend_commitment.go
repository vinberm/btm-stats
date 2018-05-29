package types

import (
	"github.com/btm-stats/protocol/bc"
	"fmt"
	"github.com/btm-stats/encoding/blockchain"
	"github.com/btm-stats/errors"
)

// SpendCommitment contains the commitment data for a transaction output.
type SpendCommitment struct {
	bc.AssetAmount
	SourceID       bc.Hash
	SourcePosition uint64
	VMVersion      uint64
	ControlProgram []byte
}

func (sc *SpendCommitment) readFrom(r *blockchain.Reader, assetVersion uint64) (suffix []byte, err error) {
	return blockchain.ReadExtensibleString(r, func(r *blockchain.Reader) error {
		if assetVersion == 1 {
			if _, err := sc.SourceID.ReadFrom(r); err != nil {
				return errors.Wrap(err, "reading source id")
			}
			if err = sc.AssetAmount.ReadFrom(r); err != nil {
				return errors.Wrap(err, "reading asset+amount")
			}
			if sc.SourcePosition, err = blockchain.ReadVarint63(r); err != nil {
				return errors.Wrap(err, "reading source position")
			}
			if sc.VMVersion, err = blockchain.ReadVarint63(r); err != nil {
				return errors.Wrap(err, "reading VM version")
			}
			if sc.VMVersion != 1 {
				return fmt.Errorf("unrecognized VM version %d for asset version 1", sc.VMVersion)
			}
			if sc.ControlProgram, err = blockchain.ReadVarstr31(r); err != nil {
				return errors.Wrap(err, "reading control program")
			}
			return nil
		}
		return nil
	})
}
