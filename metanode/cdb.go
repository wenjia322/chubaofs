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
)

var (
	InsertDBStopC = make(chan struct{}, 0)
)

func (m *MetaNode) initCdbStore(cfg *config.Config) {
	dbAddr := cfg.GetString(cfgDBAddr)
	if dbAddr != "" {
		table := fmt.Sprintf("%v_%v", m.clusterId, cdb.MetaType)
		m.cdbStore = cdb.NewCdbStore(dbAddr, table, cdb.MetaType)
		go m.startInsertDB()
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
			m.cdbStore.InsertCDB(m.nodeId)
		}
	}
}

func (m *MetaNode) stopInsertDB() {
	InsertDBStopC <- struct{}{}
}

func (m *MetaNode) gatherOpCount(p *Packet) {
	if m.cdbStore != nil && len(p.Data) > 0 {
		var req map[string]interface{}
		if err := json.Unmarshal(p.Data, &req); err != nil {
			log.LogErrorf("op monitor: json unmarshal req err[%v], data[%v]", err, string(p.Data))
			return
		}
		if vol, exist1 := req["vol"]; exist1 {
			if pid, exist2 := req["pid"]; exist2 {
				m.cdbStore.CountOpForPid(vol.(string), pid.(uint64), p.GetOpMsg())
			}
		}
	}
	return
}
