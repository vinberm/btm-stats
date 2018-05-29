package netsync

import (

	cfg "github.com/btm-stats/config"
	"github.com/bytom/protocol/bc"
	"github.com/tendermint/go-crypto"
	"github.com/bytom/p2p/pex"
	"github.com/bytom/p2p"
)

//SyncManager Sync Manager is responsible for the business layer information synchronization
type SyncManager struct {
	networkID uint64
	sw        *p2p.Switch

	privKey     crypto.PrivKeyEd25519 // local node's p2p key
	chain       *core.Chain
	txPool      *core.TxPool
	fetcher     *Fetcher
	blockKeeper *blockKeeper
	peers       *peerSet

	newBlockCh    chan *bc.Hash
	newPeerCh     chan struct{}
	txSyncCh      chan *txsync
	dropPeerCh    chan *string
	quitSync      chan struct{}
	config        *cfg.Config
	synchronising int32
}

//NewSyncManager create a sync manager
func NewSyncManager(config *cfg.Config, chain *core.Chain, txPool *core.TxPool, newBlockCh chan *bc.Hash) (*SyncManager, error) {
	// Create the protocol manager with the base fields
	manager := &SyncManager{
		txPool:     txPool,
		chain:      chain,
		privKey:    crypto.GenPrivKeyEd25519(),
		config:     config,
		quitSync:   make(chan struct{}),
		newBlockCh: newBlockCh,
		newPeerCh:  make(chan struct{}),
		txSyncCh:   make(chan *txsync),
		dropPeerCh: make(chan *string, maxQuitReq),
		peers:      newPeerSet(),
	}

	trustHistoryDB := dbm.NewDB("trusthistory", config.DBBackend, config.DBDir())
	addrBook := pex.NewAddrBook(config.P2P.AddrBookFile(), config.P2P.AddrBookStrict)
	manager.sw = p2p.NewSwitch(config.P2P, addrBook, trustHistoryDB)

	pexReactor := pex.NewPEXReactor(addrBook)
	manager.sw.AddReactor("PEX", pexReactor)

	manager.blockKeeper = newBlockKeeper(manager.chain, manager.sw, manager.peers, manager.dropPeerCh)
	manager.fetcher = NewFetcher(chain, manager.sw, manager.peers)
	protocolReactor := NewProtocolReactor(chain, txPool, manager.sw, manager.blockKeeper, manager.fetcher, manager.peers, manager.newPeerCh, manager.txSyncCh, manager.dropPeerCh)
	manager.sw.AddReactor("PROTOCOL", protocolReactor)

	// Create & add listener
	var listenerStatus bool
	var l p2p.Listener
	if !config.VaultMode {
		p, address := protocolAndAddress(manager.config.P2P.ListenAddress)
		l, listenerStatus = p2p.NewDefaultListener(p, address, manager.config.P2P.SkipUPNP)
		manager.sw.AddListener(l)
	}
	manager.sw.SetNodeInfo(manager.makeNodeInfo(listenerStatus))
	manager.sw.SetNodePrivKey(manager.privKey)

	return manager, nil
}
