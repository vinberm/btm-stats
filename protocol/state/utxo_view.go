package state

import "github.com/btm-stats/database/storage"

// UtxoViewpoint represents a view into the set of unspent transaction outputs
type UtxoViewpoint struct {
	Entries map[bc.Hash]*storage.UtxoEntry
}
