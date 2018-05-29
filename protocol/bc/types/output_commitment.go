package types

import (
	"github.com/btm-stats/protocol/bc"
	"fmt"
	"github.com/btm-stats/encoding/blockchain"
	"github.com/btm-stats/errors"
)

// OutputCommitment contains the commitment data for a transaction output.
type OutputCommitment struct {
	bc.AssetAmount
	VMVersion      uint64
	ControlProgram []byte
}


func (oc *OutputCommitment) readFrom(r *blockchain.Reader, assetVersion uint64) (suffix []byte, err error) {
	return blockchain.ReadExtensibleString(r, func(r *blockchain.Reader) error {
		if assetVersion == 1 {
			if err := oc.AssetAmount.ReadFrom(r); err != nil {
				return errors.Wrap(err, "reading asset+amount")
			}
			oc.VMVersion, err = blockchain.ReadVarint63(r)
			if err != nil {
				return errors.Wrap(err, "reading VM version")
			}
			if oc.VMVersion != 1 {
				return fmt.Errorf("unrecognized VM version %d for asset version 1", oc.VMVersion)
			}
			oc.ControlProgram, err = blockchain.ReadVarstr31(r)
			return errors.Wrap(err, "reading control program")
		}
		return nil
	})
}
