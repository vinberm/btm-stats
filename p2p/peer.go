package p2p

import "github.com/btm-stats/p2p/connection"

// Peer represent a bytom network node
type Peer struct {
	cmn.BaseService

	// raw peerConn and the multiplex connection
	*peerConn
	mconn *connection.MConnection // multiplex connection

	*NodeInfo
	Key  string
	Data *cmn.CMap // User data.
}


