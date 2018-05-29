package bc

import (
	"encoding/binary"
	"io"
	"github.com/golang/protobuf/proto"
	"reflect"
	"fmt"

	"github.com/btm-stats/errors"
	"github.com/btm-stats/encoding/blockchain"
)

// Entry is the interface implemented by each addressable unit in a
// blockchain: transaction components such as spends, issuances,
// outputs, and retirements (among others), plus blockheaders.
type Entry interface {
	proto.Message

	// type produces a short human-readable string uniquely identifying
	// the type of this entry.
	typ() string

	// writeForHash writes the entry's body for hashing.
	writeForHash(w io.Writer)
}

var byte32zero [32]byte
var errInvalidValue = errors.New("invalid value")

func writeForHash(w io.Writer, c interface{}) error {
	switch v := c.(type) {
	case byte:
		_, err := w.Write([]byte{v})
		return errors.Wrap(err, "writing byte for hash")
	case uint64:
		buf := [8]byte{}
		binary.LittleEndian.PutUint64(buf[:], v)
		_, err := w.Write(buf[:])
		return errors.Wrapf(err, "writing uint64 (%d) for hash", v)
	case []byte:
		_, err := blockchain.WriteVarstr31(w, v)
		return errors.Wrapf(err, "writing []byte (len %d) for hash", len(v))
	case [][]byte:
		_, err := blockchain.WriteVarstrList(w, v)
		return errors.Wrapf(err, "writing [][]byte (len %d) for hash", len(v))
	case string:
		_, err := blockchain.WriteVarstr31(w, []byte(v))
		return errors.Wrapf(err, "writing string (len %d) for hash", len(v))
	case *Hash:
		if v == nil {
			_, err := w.Write(byte32zero[:])
			return errors.Wrap(err, "writing nil *Hash for hash")
		}
		_, err := w.Write(v.Bytes())
		return errors.Wrap(err, "writing *Hash for hash")
	case *AssetID:
		if v == nil {
			_, err := w.Write(byte32zero[:])
			return errors.Wrap(err, "writing nil *AssetID for hash")
		}
		_, err := w.Write(v.Bytes())
		return errors.Wrap(err, "writing *AssetID for hash")
	case Hash:
		_, err := v.WriteTo(w)
		return errors.Wrap(err, "writing Hash for hash")
	case AssetID:
		_, err := v.WriteTo(w)
		return errors.Wrap(err, "writing AssetID for hash")
	}

	// The two container types in the spec (List and Struct)
	// correspond to slices and structs in Go. They can't be
	// handled with type assertions, so we must use reflect.
	switch v := reflect.ValueOf(c); v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		elem := v.Elem()
		return writeForHash(w, elem.Interface())
	case reflect.Slice:
		l := v.Len()
		if _, err := blockchain.WriteVarint31(w, uint64(l)); err != nil {
			return errors.Wrapf(err, "writing slice (len %d) for hash", l)
		}
		for i := 0; i < l; i++ {
			c := v.Index(i)
			if !c.CanInterface() {
				return errInvalidValue
			}
			if err := writeForHash(w, c.Interface()); err != nil {
				return errors.Wrapf(err, "writing slice element %d for hash", i)
			}
		}
		return nil

	case reflect.Struct:
		typ := v.Type()
		for i := 0; i < typ.NumField(); i++ {
			c := v.Field(i)
			if !c.CanInterface() {
				return errInvalidValue
			}
			if err := writeForHash(w, c.Interface()); err != nil {
				t := v.Type()
				f := t.Field(i)
				return errors.Wrapf(err, "writing struct field %d (%s.%s) for hash", i, t.Name(), f.Name)
			}
		}
		return nil
	}

	return errors.Wrap(fmt.Errorf("bad type %T", c))
}

