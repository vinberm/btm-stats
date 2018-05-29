package types

import (
	"fmt"
	"encoding/hex"
	"github.com/btm-stats/encoding/blockchain"
	"github.com/btm-stats/errors"
)

// serflag variables, start with 1
const (
	_ = iota
	SerBlockHeader
	SerBlockTransactions
	SerBlockFull
)

// Block describes a complete block, including its header and the transactions
// it contains.
type Block struct {
	BlockHeader
	Transactions []*Tx
}


// UnmarshalText fulfills the encoding.TextUnmarshaler interface.
func (b *Block) UnmarshalText(text []byte) error {
	decoded := make([]byte, hex.DecodedLen(len(text)))
	if _, err := hex.Decode(decoded, text); err != nil {
		return err
	}

	r := blockchain.NewReader(decoded)
	if err := b.readFrom(r); err != nil {
		return err
	}

	if trailing := r.Len(); trailing > 0 {
		return fmt.Errorf("trailing garbage (%d bytes)", trailing)
	}
	return nil
}

func (b *Block) readFrom(r *blockchain.Reader) error {
	serflags, err := b.BlockHeader.readFrom(r)
	if err != nil {
		return err
	}

	if serflags == SerBlockHeader {
		return nil
	}

	n, err := blockchain.ReadVarint31(r)
	if err != nil {
		return errors.Wrap(err, "reading number of transactions")
	}

	for ; n > 0; n-- {
		data := TxData{}
		if err = data.readFrom(r); err != nil {
			return errors.Wrapf(err, "reading transaction %d", len(b.Transactions))
		}

		b.Transactions = append(b.Transactions, NewTx(data))
	}
	return nil
}
