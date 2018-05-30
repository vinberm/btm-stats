package node

import (
	//"context"
	"errors"
	"time"
	"os"
	"path/filepath"

	"github.com/prometheus/prometheus/util/flock"
	log "github.com/sirupsen/logrus"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"

	cfg "github.com/btm-stats/config"
	"github.com/btm-stats/netsync"
	"github.com/btm-stats/protocol"
	"github.com/btm-stats/database/leveldb"
	"github.com/btm-stats/consensus"
	"github.com/btm-stats/protocol/bc"
)

const (
	webAddress               = "http://127.0.0.1:9888"
	expireReservationsPeriod = time.Second
	maxNewBlockChSize        = 1024
)

type Node struct {
	cmn.BaseService

	// config
	config *cfg.Config

	syncManager *netsync.SyncManager

}

func NewNode(config *cfg.Config) *Node {
	// Get store
	coreDB := dbm.NewDB("core", config.DBBackend, config.DBDir())
	store := leveldb.NewStore(coreDB)

	txPool := protocol.NewTxPool()
	chain, err := protocol.NewChain(store, txPool)
	if err != nil {
		cmn.Exit(cmn.Fmt("Failed to create chain structure: %v", err))
	}

	newBlockCh := make(chan *bc.Hash, maxNewBlockChSize)

	syncManager, _ := netsync.NewSyncManager(config, chain, txPool, newBlockCh)


	node := &Node{
		config:      config,
		syncManager: syncManager,
	}

	return node
}

// Lock data directory after daemonization
func lockDataDirectory(config *cfg.Config) error {
	_, _, err := flock.New(filepath.Join(config.RootDir, "LOCK"))
	if err != nil {
		return errors.New("datadir already used by another process")
	}
	return nil
}

func initActiveNetParams(config *cfg.Config) {
	var exist bool
	consensus.ActiveNetParams, exist = consensus.NetParams[config.ChainID]
	if !exist {
		cmn.Exit(cmn.Fmt("chain_id[%v] don't exist", config.ChainID))
	}
}

func initLogFile(config *cfg.Config) {
	if config.LogFile == "" {
		return
	}
	cmn.EnsureDir(filepath.Dir(config.LogFile), 0700)
	file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(file)
	} else {
		log.WithField("err", err).Info("using default")
	}
}

func (n *Node) OnStart() error {

	if !n.config.VaultMode {
		n.syncManager.Start()
	}

	return nil
}
