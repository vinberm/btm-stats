package p2p

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"

	cfg "github.com/btm-stats/config"
	"github.com/btm-stats/errors"
	"github.com/btm-stats/p2p/connection"
	"github.com/btm-stats/p2p/trust"
)

const (
	bannedPeerKey      = "BannedPeer"
	defaultBanDuration = time.Hour * 1
)

//pre-define errors for connecting fail
var (
	ErrDuplicatePeer     = errors.New("Duplicate peer")
	ErrConnectSelf       = errors.New("Connect self")
	ErrConnectBannedPeer = errors.New("Connect banned peer")
)

// An AddrBook represents an address book from the pex package, which is used to store peer addresses.
type AddrBook interface {
	AddAddress(*NetAddress, *NetAddress) error
	AddOurAddress(*NetAddress)
	MarkGood(*NetAddress)
	RemoveAddress(*NetAddress)
	SaveToFile() error
}

//-----------------------------------------------------------------------------

// Switch handles peer connections and exposes an API to receive incoming messages
// on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
// or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
// incoming messages are received on the reactor.
type Switch struct {
	cmn.BaseService

	Config       *cfg.P2PConfig
	peerConfig   *PeerConfig
	listeners    []Listener
	reactors     map[string]Reactor
	chDescs      []*connection.ChannelDescriptor
	reactorsByCh map[byte]Reactor
	peers        *PeerSet
	dialing      *cmn.CMap
	nodeInfo     *NodeInfo             // our node info
	nodePrivKey  crypto.PrivKeyEd25519 // our node privkey
	addrBook     AddrBook
	bannedPeer   map[string]time.Time
	db           dbm.DB
	mtx          sync.Mutex
}

// NewSwitch creates a new Switch with the given config.
func NewSwitch(config *cfg.P2PConfig, addrBook AddrBook, trustHistoryDB dbm.DB) *Switch {
	sw := &Switch{
		Config:       config,
		peerConfig:   DefaultPeerConfig(config),
		reactors:     make(map[string]Reactor),
		chDescs:      make([]*connection.ChannelDescriptor, 0),
		reactorsByCh: make(map[byte]Reactor),
		peers:        NewPeerSet(),
		dialing:      cmn.NewCMap(),
		nodeInfo:     nil,
		addrBook:     addrBook,
		db:           trustHistoryDB,
	}
	sw.BaseService = *cmn.NewBaseService(nil, "P2P Switch", sw)
	sw.bannedPeer = make(map[string]time.Time)
	if datajson := sw.db.Get([]byte(bannedPeerKey)); datajson != nil {
		if err := json.Unmarshal(datajson, &sw.bannedPeer); err != nil {
			return nil
		}
	}
	trust.Init()
	return sw
}

// AddReactor adds the given reactor to the switch.
// NOTE: Not goroutine safe.
func (sw *Switch) AddReactor(name string, reactor Reactor) Reactor {
	// Validate the reactor.
	// No two reactors can share the same channel.
	reactorChannels := reactor.GetChannels()
	for _, chDesc := range reactorChannels {
		chID := chDesc.ID
		if sw.reactorsByCh[chID] != nil {
			cmn.PanicSanity(fmt.Sprintf("Channel %X has multiple reactors %v & %v", chID, sw.reactorsByCh[chID], reactor))
		}
		sw.chDescs = append(sw.chDescs, chDesc)
		sw.reactorsByCh[chID] = reactor
	}
	sw.reactors[name] = reactor
	reactor.SetSwitch(sw)
	return reactor
}

// Reactors returns a map of reactors registered on the switch.
// NOTE: Not goroutine safe.
func (sw *Switch) Reactors() map[string]Reactor {
	return sw.reactors
}

// Reactor returns the reactor with the given name.
// NOTE: Not goroutine safe.
func (sw *Switch) Reactor(name string) Reactor {
	return sw.reactors[name]
}

// AddListener adds the given listener to the switch for listening to incoming peer connections.
// NOTE: Not goroutine safe.
func (sw *Switch) AddListener(l Listener) {
	sw.listeners = append(sw.listeners, l)
}

// Listeners returns the list of listeners the switch listens on.
// NOTE: Not goroutine safe.
func (sw *Switch) Listeners() []Listener {
	return sw.listeners
}

// IsListening returns true if the switch has at least one listener.
// NOTE: Not goroutine safe.
func (sw *Switch) IsListening() bool {
	return len(sw.listeners) > 0
}

// SetNodeInfo sets the switch's NodeInfo for checking compatibility and handshaking with other nodes.
// NOTE: Not goroutine safe.
func (sw *Switch) SetNodeInfo(nodeInfo *NodeInfo) {
	sw.nodeInfo = nodeInfo
}

// NodeInfo returns the switch's NodeInfo.
// NOTE: Not goroutine safe.
func (sw *Switch) NodeInfo() *NodeInfo {
	return sw.nodeInfo
}

// SetNodePrivKey sets the switch's private key for authenticated encryption.
// NOTE: Not goroutine safe.
func (sw *Switch) SetNodePrivKey(nodePrivKey crypto.PrivKeyEd25519) {
	sw.nodePrivKey = nodePrivKey
	if sw.nodeInfo != nil {
		sw.nodeInfo.PubKey = nodePrivKey.PubKey().Unwrap().(crypto.PubKeyEd25519)
	}
}

// OnStart implements BaseService. It starts all the reactors, peers, and listeners.
func (sw *Switch) OnStart() error {
	// Start reactors
	for _, reactor := range sw.reactors {
		_, err := reactor.Start()
		if err != nil {
			return err
		}
	}
	// Start listeners
	for _, listener := range sw.listeners {
		go sw.listenerRoutine(listener)
	}
	return nil
}

// OnStop implements BaseService. It stops all listeners, peers, and reactors.
func (sw *Switch) OnStop() {
	// Stop listeners
	for _, listener := range sw.listeners {
		listener.Stop()
	}
	sw.listeners = nil
	// Stop peers
	for _, peer := range sw.peers.List() {
		peer.Stop()
		sw.peers.Remove(peer)
	}
	// Stop reactors
	for _, reactor := range sw.reactors {
		reactor.Stop()
	}
}

// AddPeer performs the P2P handshake with a peer
// that already has a SecretConnection. If all goes well,
// it starts the peer and adds it to the switch.
// NOTE: This performs a blocking handshake before the peer is added.
// CONTRACT: If error is returned, peer is nil, and conn is immediately closed.
func (sw *Switch) AddPeer(pc *peerConn) error {
	peerNodeInfo, err := pc.HandshakeTimeout(sw.nodeInfo, time.Duration(sw.peerConfig.HandshakeTimeout*time.Second))
	if err != nil {
		return err
	}
	// Check version, chain id
	if err := sw.nodeInfo.CompatibleWith(peerNodeInfo); err != nil {
		return err
	}

	peer := newPeer(pc, peerNodeInfo, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError)

	//filter peer
	if err := sw.filterConnByPeer(peer); err != nil {
		return err
	}

	// Start peer
	if sw.IsRunning() {
		if err := sw.startInitPeer(peer); err != nil {
			return err
		}
	}

	// Add the peer to .peers.
	// We start it first so that a peer in the list is safe to Stop.
	// It should not err since we already checked peers.Has()
	if err := sw.peers.Add(peer); err != nil {
		return err
	}

	log.Info("Added peer:", peer)
	return nil
}

func (sw *Switch) startInitPeer(peer *Peer) error {
	peer.Start() // spawn send/recv routines
	for _, reactor := range sw.reactors {
		if err := reactor.AddPeer(peer); err != nil {
			return err
		}
	}
	return nil
}

func (sw *Switch) dialSeed(addr *NetAddress) {
	err := sw.DialPeerWithAddress(addr)
	if err != nil {
		log.Info("Error dialing seed:", addr.String())
	}
}

func (sw *Switch) addrBookDelSelf() error {
	addr, err := NewNetAddressString(sw.nodeInfo.ListenAddr)
	if err != nil {
		return err
	}
	// remove the given address from the address book if we're added it earlier
	sw.addrBook.RemoveAddress(addr)
	// add the given address to the address book to avoid dialing ourselves
	// again this is our public address
	sw.addrBook.AddOurAddress(addr)
	return nil
}

func (sw *Switch) filterConnByIP(ip string) error {
	if err := sw.checkBannedPeer(ip); err != nil {
		return ErrConnectBannedPeer
	}

	if ip == sw.nodeInfo.ListenHost() {
		sw.addrBookDelSelf()
		return ErrConnectSelf
	}

	return nil
}

func (sw *Switch) filterConnByPeer(peer *Peer) error {
	if err := sw.checkBannedPeer(peer.RemoteAddrHost()); err != nil {
		return ErrConnectBannedPeer
	}

	if sw.nodeInfo.PubKey.Equals(peer.PubKey().Wrap()) {
		sw.addrBookDelSelf()
		return ErrConnectSelf
	}

	// Check for duplicate peer
	if sw.peers.Has(peer.Key) {
		return ErrDuplicatePeer
	}
	return nil
}

//DialPeerWithAddress dial node from net address
func (sw *Switch) DialPeerWithAddress(addr *NetAddress) error {
	log.Debug("Dialing peer address:", addr)

	if err := sw.filterConnByIP(addr.IP.String()); err != nil {
		return err
	}

	sw.dialing.Set(addr.IP.String(), addr)
	defer sw.dialing.Delete(addr.IP.String())

	pc, err := newOutboundPeerConn(addr, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey, sw.peerConfig)
	if err != nil {
		log.Debug("Failed to dial peer", " address:", addr, " error:", err)
		return err
	}

	err = sw.AddPeer(pc)
	if err != nil {
		log.Info("Failed to add peer:", addr, " err:", err)
		pc.CloseConn()
		return err
	}
	log.Info("Dialed and added peer:", addr)
	return nil
}

//IsDialing prevent duplicate dialing
func (sw *Switch) IsDialing(addr *NetAddress) bool {
	return sw.dialing.Has(addr.IP.String())
}

// NumPeers Returns the count of outbound/inbound and outbound-dialing peers.
func (sw *Switch) NumPeers() (outbound, inbound, dialing int) {
	peers := sw.peers.List()
	for _, peer := range peers {
		if peer.outbound {
			outbound++
		} else {
			inbound++
		}
	}
	dialing = sw.dialing.Size()
	return
}

//Peers return switch peerset
func (sw *Switch) Peers() *PeerSet {
	return sw.peers
}

// StopPeerForError disconnects from a peer due to external error.
func (sw *Switch) StopPeerForError(peer *Peer, reason interface{}) {
	log.Info("Stopping peer for error.", " peer:", peer, " err:", reason)
	sw.stopAndRemovePeer(peer, reason)
}

// StopPeerGracefully disconnect from a peer gracefully.
func (sw *Switch) StopPeerGracefully(peer *Peer) {
	log.Info("Stopping peer gracefully")
	sw.stopAndRemovePeer(peer, nil)
}

func (sw *Switch) stopAndRemovePeer(peer *Peer, reason interface{}) {
	for _, reactor := range sw.reactors {
		reactor.RemovePeer(peer, reason)
	}
	sw.peers.Remove(peer)
	peer.Stop()
}

func (sw *Switch) listenerRoutine(l Listener) {
	for {
		inConn, ok := <-l.Connections()
		if !ok {
			break
		}

		// disconnect if we alrady have 2 * MaxNumPeers, we do this because we wanna address book get exchanged even if
		// the connect is full. The pex will disconnect the peer after address exchange, the max connected peer won't
		// be double of MaxNumPeers
		if sw.peers.Size() >= sw.Config.MaxNumPeers*2 {
			inConn.Close()
			log.Info("Ignoring inbound connection: already have enough peers.")
			continue
		}

		// New inbound connection!
		err := sw.addPeerWithConnection(inConn)
		if err != nil {
			log.Info("Ignoring inbound connection: error while adding peer.", " address:", inConn.RemoteAddr().String(), " error:", err)
			continue
		}
	}
}

func (sw *Switch) addPeerWithConnection(conn net.Conn) error {
	peerConn, err := newInboundPeerConn(conn, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError, sw.nodePrivKey, sw.Config)
	if err != nil {
		conn.Close()
		return err
	}
	if err = sw.AddPeer(peerConn); err != nil {
		conn.Close()
		return err
	}

	return nil
}

//AddBannedPeer add peer to blacklist
func (sw *Switch) AddBannedPeer(peer *Peer) error {
	sw.mtx.Lock()
	defer sw.mtx.Unlock()
	key := peer.NodeInfo.RemoteAddrHost()
	sw.bannedPeer[key] = time.Now().Add(defaultBanDuration)
	datajson, err := json.Marshal(sw.bannedPeer)
	if err != nil {
		return err
	}
	sw.db.Set([]byte(bannedPeerKey), datajson)
	return nil
}

func (sw *Switch) delBannedPeer(addr string) error {
	delete(sw.bannedPeer, addr)
	datajson, err := json.Marshal(sw.bannedPeer)
	if err != nil {
		return err
	}
	sw.db.Set([]byte(bannedPeerKey), datajson)
	return nil
}

func (sw *Switch) checkBannedPeer(peer string) error {
	sw.mtx.Lock()
	defer sw.mtx.Unlock()

	if banEnd, ok := sw.bannedPeer[peer]; ok {
		if time.Now().Before(banEnd) {
			return ErrConnectBannedPeer
		}
		sw.delBannedPeer(peer)
	}
	return nil
}
