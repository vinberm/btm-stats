package netsync

import (
	log "github.com/sirupsen/logrus"
	"sync"
	"github.com/btm-stats/errors"
	"github.com/btm-stats/protocol/bc"
	"github.com/btm-stats/protocol/bc/types"
	"github.com/btm-stats/p2p"
	"github.com/btm-stats/p2p/trust"
	"gopkg.in/karalabe/cookiejar.v2/collections/set"
)

const (
	defaultVersion      = 1
	defaultBanThreshold = uint64(100)
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

// Peer retrieves the registered peer with the given id.
func (ps *peerSet) Peer(id string) (*peer, bool) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	p, ok := ps.peers[id]
	return p, ok
}

func (p *peer) getPeer() *p2p.Peer {
	p.mtx.RLock()
	defer p.mtx.RUnlock()

	return p.swPeer
}

func (ps *peerSet) BroadcastMinedBlock(block *types.Block) ([]*peer, error) {
	msg, err := NewMinedBlockMessage(block)
	if err != nil {
		return nil, errors.New("Failed construction block msg")
	}
	hash := block.Hash()
	peers := ps.PeersWithoutBlock(&hash)
	abnormalPeers := make([]*peer, 0)
	for _, peer := range peers {
		if ok := peer.swPeer.TrySend(BlockchainChannel, struct{ BlockchainMessage }{msg}); !ok {
			abnormalPeers = append(abnormalPeers, peer)
			continue
		}
		if p, ok := ps.Peer(peer.id); ok {
			p.MarkBlock(&hash)
		}
	}
	return abnormalPeers, nil
}

// addBanScore increases the persistent and decaying ban score fields by the
// values passed as parameters. If the resulting score exceeds half of the ban
// threshold, a warning is logged including the reason provided. Further, if
// the score is above the ban threshold, the peer will be banned and
// disconnected.
func (p *peer) addBanScore(persistent, transient uint64, reason string) bool {
	warnThreshold := defaultBanThreshold >> 1
	if transient == 0 && persistent == 0 {
		// The score is not being increased, but a warning message is still
		// logged if the score is above the warn threshold.
		score := p.banScore.Int()
		if score > warnThreshold {
			log.Infof("Misbehaving peer %s: %s -- ban score is %d, "+"it was not increased this time", p.id, reason, score)
		}
		return false
	}
	score := p.banScore.Increase(persistent, transient)
	if score > warnThreshold {
		log.Infof("Misbehaving peer %s: %s -- ban score increased to %d", p.id, reason, score)
		if score > defaultBanThreshold {
			log.Errorf("Misbehaving peer %s -- banning and disconnecting", p.id)
			return true
		}
	}
	return false
}

// BestPeer retrieves the known peer with the currently highest total difficulty.
func (ps *peerSet) BestPeer() (*p2p.Peer, uint64) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	var bestPeer *p2p.Peer
	var bestHeight uint64

	for _, p := range ps.peers {
		if bestPeer == nil || p.height > bestHeight {
			bestPeer, bestHeight = p.swPeer, p.height
		}
	}

	return bestPeer, bestHeight
}

// newPeerSet creates a new peer set to track the active participants.
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*peer),
	}
}
