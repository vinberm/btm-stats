package netsync

import (
	"sync"
	"errors"
	"gopkg.in/fatih/set.v0"

	"github.com/btm-stats/protocol/bc"
	"github.com/btm-stats/p2p"
	"github.com/btm-stats/p2p/trust"
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
)

type peer struct {
	mtx      sync.RWMutex
	version  int // Protocol version negotiated
	id       string
	height   uint64
	hash     *bc.Hash
	banScore trust.DynamicBanScore

	swPeer *p2p.Peer

	knownTxs    *set.Set // Set of transaction hashes known to be known by this peer
	knownBlocks *set.Set // Set of block hashes known to be known by this peer
}

type peerSet struct {
	peers  map[string]*peer
	lock   sync.RWMutex
	closed bool
}
