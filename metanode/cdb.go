package metanode

import (
	"encoding/json"
	"fmt"
	"time"

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

func (m *MetaNode) initCdbStore(cfg *config.Config) {
	dbAddr := cfg.GetString(cfgDBAddr)
	if dbAddr != "" {
		table := fmt.Sprintf("%v_%v", m.clusterId, cdb.MetaType)
		m.cdbStore = cdb.NewCdbStore(dbAddr, table, cdb.MetaType)
		go m.startInsertDB()
		go m.startClearVolOp()
	}
	log.LogDebugf("action[initCdbStore] load ChubaoDB config(%v).", m.cdbStore)
}

func (m *MetaNode) startInsertDB() {
	ticker := time.NewTicker(InsertDBTicket)
	defer ticker.Stop()
	for {
		select {
		case <-InsertDBStopC:
			log.LogInfo("metanode insert chubaodb goroutine stopped")
			return
		case <-ticker.C:
			m.cdbStore.InsertCDB()
		}
	}
}

func (m *MetaNode) stopInsertDB() {
	InsertDBStopC <- struct{}{}
}

func (m *MetaNode) startClearVolOp() {
	ticker := time.NewTicker(ClearVolTicket)
	defer ticker.Stop()
	for {
		select {
		case <-ClearVolOpStopC:
			log.LogInfo("metanode clear vol of op count goroutine stopped")
			return
		case <-ticker.C:
			m.cdbStore.ClearVol()
		}
	}
}

func (m *MetaNode) stopClearVolOp() {
	ClearVolOpStopC <- struct{}{}
}

func (m *MetaNode) gatherOpCount(p *Packet) {
	if m.cdbStore != nil && len(p.Data) > 0 {
		if _, exist := cdb.MetaOps[p.Opcode]; exist {
			var req map[string]interface{}
			if err := json.Unmarshal(p.Data, &req); err != nil {
				log.LogErrorf("op monitor: json unmarshal req err[%v], data[%v]", err, string(p.Data))
				return
			}
			if vol, exist1 := req["vol"]; exist1 {
				if pid, exist2 := req["pid"]; exist2 {
					go m.cdbStore.CountOpForPid(vol, pid, p.GetOpMsg())
				}
			}
		}
	}
	return
}
