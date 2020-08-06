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
	ClearVolTicket = 7 * 24 * time.Hour // a week
)

var (
	InsertDBStopC   = make(chan struct{}, 0)
	ClearVolOpStopC = make(chan struct{}, 0)
)

func (s *DataNode) initCdbStore(cfg *config.Config) {
	dbAddr := cfg.GetString(ConfigKeyDBAddr)
	if dbAddr != "" {
		table := fmt.Sprintf("%v_%v", s.clusterID, cdb.DataType)
		s.cdbStore = cdb.NewCdbStore(dbAddr, table, cdb.DataType, s.nodeID)
		go s.startInsertDB()
		go s.startClearVolOp()
	}
	log.LogDebugf("action[initCdbStore] load ChubaoDB config(%v).", s.cdbStore)
}

func (s *DataNode) gatherOpCount(p *repl.Packet) {
	if s.cdbStore != nil && p.Object != nil {
		if _, exist := cdb.DataOps[p.Opcode]; exist {
			dp := p.Object.(*DataPartition)
			go s.cdbStore.CountOpForPid(dp.volumeID, dp.partitionID, p.GetOpMsg())
		}
	}
}

func (s *DataNode) startInsertDB() {
	ticker := time.NewTicker(InsertDBTicket)
	defer ticker.Stop()
	for {
		select {
		case <-InsertDBStopC:
			log.LogInfo("datanode insert chubaodb goroutine stopped")
			return
		case <-ticker.C:
			s.cdbStore.InsertCDB()
		}
	}
}

func (s *DataNode) stopInsertDB() {
	InsertDBStopC <- struct{}{}
}

func (s *DataNode) startClearVolOp() {
	ticker := time.NewTicker(ClearVolTicket)
	defer ticker.Stop()
	for {
		select {
		case <-ClearVolOpStopC:
			log.LogInfo("datanode clear vol of op count goroutine stopped")
			return
		case <-ticker.C:
			s.cdbStore.ClearVol()
		}
	}
}

func (s *DataNode) stopClearVolOp() {
	ClearVolOpStopC <- struct{}{}
}
