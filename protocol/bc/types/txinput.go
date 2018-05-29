package types

import (
	"fmt"
	"io"

	"github.com/btm-stats/encoding/blockchain"
	"github.com/btm-stats/errors"
	"github.com/btm-stats/protocol/bc"
)

// serflag variables for input types.
const (
	IssuanceInputType uint8 = iota
	SpendInputType
	CoinbaseInputType
)

type (
	// TxInput is the top level struct of tx input.
	TxInput struct {
		AssetVersion uint64
		TypedInput
		CommitmentSuffix []byte
		WitnessSuffix    []byte
	}

	// TypedInput return the txinput type.
	TypedInput interface {
		InputType() uint8
	}
)

var errBadAssetID = errors.New("asset ID does not match other issuance parameters")

func (t *TxInput) readFrom(r *blockchain.Reader) (err error) {
	if t.AssetVersion, err = blockchain.ReadVarint63(r); err != nil {
		return err
	}

	var assetID bc.AssetID
	t.CommitmentSuffix, err = blockchain.ReadExtensibleString(r, func(r *blockchain.Reader) error {
		if t.AssetVersion != 1 {
			return nil
		}
		var icType [1]byte
		if _, err = io.ReadFull(r, icType[:]); err != nil {
			return errors.Wrap(err, "reading input commitment type")
		}
		switch icType[0] {
		case IssuanceInputType:
			ii := new(IssuanceInput)
			t.TypedInput = ii

			if ii.Nonce, err = blockchain.ReadVarstr31(r); err != nil {
				return err
			}
			if _, err = assetID.ReadFrom(r); err != nil {
				return err
			}
			if ii.Amount, err = blockchain.ReadVarint63(r); err != nil {
				return err
			}

		case SpendInputType:
			si := new(SpendInput)
			t.TypedInput = si
			if si.SpendCommitmentSuffix, err = si.SpendCommitment.readFrom(r, 1); err != nil {
				return err
			}

		case CoinbaseInputType:
			ci := new(CoinbaseInput)
			t.TypedInput = ci
			if ci.Arbitrary, err = blockchain.ReadVarstr31(r); err != nil {
				return err
			}

		default:
			return fmt.Errorf("unsupported input type %d", icType[0])
		}
		return nil
	})
	if err != nil {
		return err
	}

	t.WitnessSuffix, err = blockchain.ReadExtensibleString(r, func(r *blockchain.Reader) error {
		if t.AssetVersion != 1 {
			return nil
		}

		switch inp := t.TypedInput.(type) {
		case *IssuanceInput:
			if inp.AssetDefinition, err = blockchain.ReadVarstr31(r); err != nil {
				return err
			}
			if inp.VMVersion, err = blockchain.ReadVarint63(r); err != nil {
				return err
			}
			if inp.IssuanceProgram, err = blockchain.ReadVarstr31(r); err != nil {
				return err
			}
			if inp.AssetID() != assetID {
				return errBadAssetID
			}
			if inp.Arguments, err = blockchain.ReadVarstrList(r); err != nil {
				return err
			}

		case *SpendInput:
			if inp.Arguments, err = blockchain.ReadVarstrList(r); err != nil {
				return err
			}
		}
		return nil
	})

	return err
}
