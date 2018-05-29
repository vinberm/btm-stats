package netsync

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gopkg.in/karalabe/cookiejar.v2/collections/prque"

	"github.com/btm-stats/p2p"
	core "github.com/btm-stats/protocol"
	"github.com/btm-stats/protocol/bc"
	"github.com/btm-stats/protocol/bc/types"
)

const (
	maxQueueDist = 1024 //32 // Maximum allowed distance from the chain head to queue
)

var (
	errTerminated = errors.New("terminated")
)

// Fetcher is responsible for accumulating block announcements from various peers
// and scheduling them for retrieval.
type Fetcher struct {
	chain *core.Chain
	sw    *p2p.Switch
	peers *peerSet

	// Various event channels
	newMinedBlock chan *blockPending
	quit          chan struct{}

	// Block cache
	queue  *prque.Prque              // Queue containing the import operations (block number sorted)
	queues map[string]int            // Per peer block counts to prevent memory exhaustion
	queued map[bc.Hash]*blockPending // Set of already queued blocks (to dedup imports)
}

//NewFetcher New creates a block fetcher to retrieve blocks of the new mined.
func NewFetcher(chain *core.Chain, sw *p2p.Switch, peers *peerSet) *Fetcher {
	return &Fetcher{
		chain:         chain,
		sw:            sw,
		peers:         peers,
		newMinedBlock: make(chan *blockPending),
		quit:          make(chan struct{}),
		queue:         prque.New(),
		queued:        make(map[bc.Hash]*blockPending),
	}
}

// Start boots up the announcement based synchroniser, accepting and processing
// hash notifications and block fetches until termination requested.
func (f *Fetcher) Start() {
	go f.loop()
}

// Stop terminates the announcement based synchroniser, canceling all pending
// operations.
func (f *Fetcher) Stop() {
	close(f.quit)
}

// Enqueue tries to fill gaps the the fetcher's future import queue.
func (f *Fetcher) Enqueue(peer string, block *types.Block) error {
	op := &blockPending{
		peerID: peer,
		block:  block,
	}
	select {
	case f.newMinedBlock <- op:
		return nil
	case <-f.quit:
		return errTerminated
	}
}

// Loop is the main fetcher loop, checking and processing various notification
// events.
func (f *Fetcher) loop() {
	for {
		// Import any queued blocks that could potentially fit
		height := f.chain.BestBlockHeight()
		for !f.queue.Empty() {
			op := f.queue.PopItem().(*blockPending)
			// If too high up the chain or phase, continue later
			number := op.block.Height
			if number > height+1 {
				f.queue.Push(op, -float32(op.block.Height))
				break
			}
			// Otherwise if fresh and still unknown, try and import
			hash := op.block.Hash()
			block, _ := f.chain.GetBlockByHash(&hash)
			if block != nil {
				f.forgetBlock(hash)
				continue
			}
			if op.block.PreviousBlockHash.String() != f.chain.BestBlockHash().String() {
				f.forgetBlock(hash)
				continue
			}
			f.insert(op.peerID, op.block)
		}
		// Wait for an outside event to occur
		select {
		case <-f.quit:
			// Fetcher terminating, abort all operations
			return

		case op := <-f.newMinedBlock:
			// A direct block insertion was requested, try and fill any pending gaps
			f.enqueue(op.peerID, op.block)
		}
	}
}

// enqueue schedules a new future import operation, if the block to be imported
// has not yet been seen.
func (f *Fetcher) enqueue(peer string, block *types.Block) {
	hash := block.Hash()

	//TODO: Ensure the peer isn't DOSing us
	// Discard any past or too distant blocks
	if dist := int64(block.Height) - int64(f.chain.BestBlockHeight()); dist < 0 || dist > maxQueueDist {
		log.Info("Discarded propagated block, too far away", " peer: ", peer, "number: ", block.Height, "distance: ", dist)
		return
	}
	// Schedule the block for future importing
	if _, ok := f.queued[hash]; !ok {
		op := &blockPending{
			peerID: peer,
			block:  block,
		}
		f.queued[hash] = op
		f.queue.Push(op, -float32(block.Height))
		log.Info("Queued receive mine block.", " peer:", peer, " number:", block.Height, " queued:", f.queue.Size())
	}
}

// insert spawns a new goroutine to run a block insertion into the chain. If the
// block's number is at the same height as the current import phase, it updates
// the phase states accordingly.
func (f *Fetcher) insert(peerID string, block *types.Block) {
	// Run the import on a new thread
	log.Info("Importing propagated block", " from peer: ", peerID, " height: ", block.Height)
	// Run the actual import and log any issues
	if _, err := f.chain.ProcessBlock(block); err != nil {
		log.Info("Propagated block import failed", " from peer: ", peerID, " height: ", block.Height, "err: ", err)
		fPeer, ok := f.peers.Peer(peerID)
		if !ok {
			return
		}
		swPeer := fPeer.getPeer()
		if ban := fPeer.addBanScore(20, 0, "block process error"); ban {
			f.sw.AddBannedPeer(swPeer)
			f.sw.StopPeerGracefully(swPeer)
		}
		return
	}
	// If import succeeded, broadcast the block
	log.Info("success process a block from new mined blocks cache. block height: ", block.Height)
	peers, err := f.peers.BroadcastMinedBlock(block)
	if err != nil {
		log.Errorf("Broadcast mine block error. %v", err)
		return
	}
	for _, fPeer := range peers {
		if fPeer == nil {
			continue
		}
		swPeer := fPeer.getPeer()
		log.Info("Fetcher broadcast block error. Stop peer.")
		f.sw.StopPeerGracefully(swPeer)
	}
}

// forgetBlock removes all traces of a queued block from the fetcher's internal
// state.
func (f *Fetcher) forgetBlock(hash bc.Hash) {
	if insert := f.queued[hash]; insert != nil {
		delete(f.queued, hash)
	}
}
