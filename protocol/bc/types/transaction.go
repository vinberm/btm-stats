package types

import (
	"github.com/btm-stats/protocol/bc"
	"github.com/btm-stats/encoding/blockchain"
	"github.com/btm-stats/errors"

	"io"
	"fmt"
)

const serRequired = 0x7 // Bit mask accepted serialization flag.

// Tx holds a transaction along with its hash.
type Tx struct {
	TxData
	*bc.Tx `json:"-"`
}


// TxData encodes a transaction in the blockchain.
type TxData struct {
	Version        uint64
	SerializedSize uint64
	TimeRange      uint64
	Inputs         []*TxInput
	Outputs        []*TxOutput
}

// NewTx returns a new Tx containing data and its hash. If you have already
// computed the hash, use struct literal notation to make a Tx object directly.
func NewTx(data TxData) *Tx {
	return &Tx{
		TxData: data,
		Tx:     MapTx(&data),
	}
}


func (tx *TxData) readFrom(r *blockchain.Reader) (err error) {
	startSerializedSize := r.Len()
	var serflags [1]byte
	if _, err = io.ReadFull(r, serflags[:]); err != nil {
		return errors.Wrap(err, "reading serialization flags")
	}
	if serflags[0] != serRequired {
		return fmt.Errorf("unsupported serflags %#x", serflags[0])
	}

	if tx.Version, err = blockchain.ReadVarint63(r); err != nil {
		return errors.Wrap(err, "reading transaction version")
	}
	if tx.TimeRange, err = blockchain.ReadVarint63(r); err != nil {
		return err
	}

	n, err := blockchain.ReadVarint31(r)
	if err != nil {
		return errors.Wrap(err, "reading number of transaction inputs")
	}

	for ; n > 0; n-- {
		ti := new(TxInput)
		if err = ti.readFrom(r); err != nil {
			return errors.Wrapf(err, "reading input %d", len(tx.Inputs))
		}
		tx.Inputs = append(tx.Inputs, ti)
	}

	n, err = blockchain.ReadVarint31(r)
	if err != nil {
		return errors.Wrap(err, "reading number of transaction outputs")
	}

	for ; n > 0; n-- {
		to := new(TxOutput)
		if err = to.readFrom(r); err != nil {
			return errors.Wrapf(err, "reading output %d", len(tx.Outputs))
		}
		tx.Outputs = append(tx.Outputs, to)
	}
	tx.SerializedSize = uint64(startSerializedSize - r.Len())
	return nil
}
