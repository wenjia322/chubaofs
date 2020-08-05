package cdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/chubaofs/chubaofs/proto"
	"github.com/chubaofs/chubaofs/util/log"
)

type NodeType string

const (
	MetaType NodeType = "meta"
	DataType NodeType = "data"

	FieldVol  = "vol_name"
	FieldPid  = "partition_id"
	FieldTime = "insert_time"
)

type CdbStore struct {
	Addr       string
	Table      string
	Type       NodeType
	VolOpCount sync.Map // key: "vol" string, value: op count map[string]int
}

type volPidOp struct {
	vol     string
	pid     string
	opCount map[string]int
	hits    int // Statistics
}

var DataOps = map[uint8]string{
	proto.OpCreateExtent:       "OpCreateExtent",
	proto.OpBatchDeleteExtent:  "OpBatchDeleteExtent",
	proto.OpStreamRead:         "OpStreamRead",
	proto.OpRead:               "Read",
	proto.OpStreamFollowerRead: "OpStreamFollowerRead",
	proto.OpWrite:              "OpWrite",
	proto.OpRandomWrite:        "OpRandomWrite",
	proto.OpSyncRandomWrite:    "OpSyncRandomWrite",
	proto.OpSyncWrite:          "OpSyncWrite",
	proto.OpMarkDelete:         "OpMarkDelete",
}

var MetaOps = map[uint8]string{
	proto.OpMetaCreateInode:   "OpMetaCreateInode",
	proto.OpMetaUnlinkInode:   "OpMetaUnlinkInode",
	proto.OpMetaCreateDentry:  "OpMetaCreateDentry",
	proto.OpMetaDeleteDentry:  "OpMetaDeleteDentry",
	proto.OpMetaLookup:        "OpMetaLookup",
	proto.OpMetaReadDir:       "OpMetaReadDir",
	proto.OpMetaInodeGet:      "OpMetaInodeGet",
	proto.OpMetaBatchInodeGet: "OpMetaBatchInodeGet",
	proto.OpMetaExtentsAdd:    "OpMetaExtentsAdd",
	proto.OpMetaExtentsDel:    "OpMetaExtentsDel",
	proto.OpMetaExtentsList:   "OpMetaExtentsList",
	proto.OpMetaUpdateDentry:  "OpMetaUpdateDentry",
	proto.OpMetaTruncate:      "OpMetaTruncate",
	proto.OpMetaLinkInode:     "OpMetaLinkInode",
	proto.OpMetaEvictInode:    "OpMetaEvictInode",
	proto.OpMetaSetattr:       "OpMetaSetattr",
}

func NewCdbStore(addr, table string, nodeType NodeType) *CdbStore {
	return &CdbStore{Addr: addr, Table: table, Type: nodeType}
}

func NewDataOpCountMap() map[string]int {
	m := make(map[string]int)
	for _, msg := range DataOps {
		m[msg] = 0
	}
	return m
}

func NewMetaOpCountMap() map[string]int {
	m := make(map[string]int)
	for _, msg := range MetaOps {
		m[msg] = 0
	}
	return m
}

func (cdb *CdbStore) CountOp(vol, op string) {
	var opMap map[string]int
	if v, ok := cdb.VolOpCount.Load(vol); ok {
		opMap = v.(map[string]int)
	} else {
		switch cdb.Type {
		case MetaType:
			opMap = NewMetaOpCountMap()
		case DataType:
			opMap = NewDataOpCountMap()
		default:
			return
		}
	}
	if count, exist := opMap[op]; exist {
		opMap[op] = count + 1
	}
	cdb.VolOpCount.Store(vol, opMap)
}

func (cdb *CdbStore) CountOpForPid(vol, pid interface{}, op string) {
	var volOp *volPidOp
	key := fmt.Sprintf("%v_%v", vol, pid)
	partitionId := fmt.Sprintf("%v", pid)
	if v, ok := cdb.VolOpCount.Load(key); ok {
		volOp = v.(*volPidOp)
	} else {
		volOp = &volPidOp{vol: vol.(string), pid: partitionId, hits: 0}
		switch cdb.Type {
		case MetaType:
			volOp.opCount = NewMetaOpCountMap()
		case DataType:
			volOp.opCount = NewDataOpCountMap()
		default:
			return
		}
	}
	if count, exist := volOp.opCount[op]; exist {
		volOp.opCount[op] = count + 1
	}
	volOp.hits = volOp.hits + 1
	cdb.VolOpCount.Store(key, volOp)
}

func clearOpCount(opCountMap map[string]int) {
	for k := range opCountMap {
		opCountMap[k] = 0
	}
}

func (cdb *CdbStore) InsertCDB() {
	var (
		body []byte
		err  error
	)
	timestamp := time.Now().Unix()
	cdb.VolOpCount.Range(func(key, value interface{}) bool {
		id := fmt.Sprintf("%v_%v", key, timestamp)
		url := fmt.Sprintf("http://%v/put/%v/%v", cdb.Addr, cdb.Table, id)
		volOp := value.(*volPidOp)
		item := make(map[string]interface{})
		for k, v := range volOp.opCount {
			item[k] = v
		}
		item[FieldVol] = volOp.vol
		item[FieldPid] = volOp.pid
		item[FieldTime] = timestamp
		clearOpCount(volOp.opCount)
		if body, err = json.Marshal(item); err != nil {
			log.LogErrorf("insert chubaodb: json marshal err[%v], data[%v]", err, body)
			return true
		}
		sendRequest(url, body)
		return true
	})
}

func (cdb *CdbStore) ClearVol() {
	cdb.VolOpCount.Range(func(key, value interface{}) bool {
		volOp := value.(*volPidOp)
		if volOp.hits == 0 {
			cdb.VolOpCount.Delete(key)
		} else {
			volOp.hits = 0
		}
		return true
	})
}

func sendRequest(url string, body []byte) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.LogErrorf("send chubaodb insert request[%s]: new request err: [%s]", url, err.Error())
		return
	}

	do, err := http.DefaultClient.Do(req)
	if err != nil {
		log.LogErrorf("send chubaodb insert request[%s]: do client err: [%s]", url, err.Error())
		return
	}

	if do.StatusCode != 200 {
		log.LogWarnf("send chubaodb insert request[%s]: status code: [%v]", url, do.StatusCode)
		return
	}

	all, err := ioutil.ReadAll(do.Body)
	_ = do.Body.Close()
	if err != nil {
		log.LogErrorf("send chubaodb insert request[%s]: read body err: [%s]", url, err.Error())
		return
	}

	var resp = &struct {
		Code int32  `json:"code"`
		Msg  string `json:"msg"`
		Data []byte `json:"data"`
	}{}
	if err = json.Unmarshal(all, &resp); err != nil {
		log.LogErrorf("send chubaodb insert request[%s]: unmarshal err: [%s]", url, err.Error())
		return
	}

	if resp.Code > 200 {
		log.LogWarnf("send chubaodb insert request[%s]: response code: [%v]", url, resp.Code)
		return
	}

	return
}
