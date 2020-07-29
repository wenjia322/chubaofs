package datanode

import (
	"fmt"
	"time"

	"github.com/chubaofs/chubaofs/repl"
	"github.com/chubaofs/chubaofs/util/cdb"
	"github.com/chubaofs/chubaofs/util/config"
	"github.com/chubaofs/chubaofs/util/log"
)

const (
	InsertDBTicket = 10 * time.Second
)

var (
	InsertDBStopC = make(chan struct{}, 0)
)

func (s *DataNode) initCdbStore(cfg *config.Config) {
	dbAddr := cfg.GetString(ConfigKeyDBAddr)
	if dbAddr != "" {
		table := fmt.Sprintf("%v_%v", s.clusterID, cdb.DataType)
		s.cdbStore = cdb.NewCdbStore(dbAddr, table, cdb.DataType)
		go s.startInsertDB()
	}
	log.LogDebugf("action[initCdbStore] load ChubaoDB config(%v).", s.cdbStore)
}

func (s *DataNode) gatherOpCount(p *repl.Packet) {
	if s.cdbStore != nil && p.Object != nil {
		dp := p.Object.(*DataPartition)
		s.cdbStore.CountOp(dp.volumeID, p.GetOpMsg())
	}
}

func (m *DataNode) startInsertDB() {
	ticker := time.NewTicker(InsertDBTicket)
	defer ticker.Stop()
	for {
		select {
		case <-InsertDBStopC:
			log.LogInfo("datanode insert chubaodb goroutine stopped")
			return
		case <-ticker.C:
			m.cdbStore.InsertCDB(m.nodeID)
		}
	}
}

func (m *DataNode) stopInsertDB() {
	InsertDBStopC <- struct{}{}
}
