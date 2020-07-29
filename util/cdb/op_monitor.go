package cdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

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

func NewCdbStore(addr, table string, nodeType NodeType) *CdbStore {
	return &CdbStore{Addr: addr, Table: table, Type: nodeType}
}

func NewDataOpCountMap() map[string]int {
	return map[string]int{
		"OpCreateExtent":       0,
		"OpMarkDelete":         0,
		"OpWrite":              0,
		"OpRead":               0,
		"OpStreamRead":         0,
		"OpStreamFollowerRead": 0,
		"OpRandomWrite":        0,
		"OpSyncRandomWrite":    0,
		"OpSyncWrite":          0,
		"OpBatchDeleteExtent":  0,
	}
}

func NewMetaOpCountMap() map[string]int {
	return map[string]int{
		"OpMetaCreateInode":   0,
		"OpMetaUnlinkInode":   0,
		"OpMetaCreateDentry":  0,
		"OpMetaDeleteDentry":  0,
		"OpMetaLookup":        0,
		"OpMetaReadDir":       0,
		"OpMetaInodeGet":      0,
		"OpMetaBatchInodeGet": 0,
		"OpMetaExtentsAdd":    0,
		"OpMetaExtentsDel":    0,
		"OpMetaExtentsList":   0,
		"OpMetaUpdateDentry":  0,
		"OpMetaTruncate":      0,
		"OpMetaLinkInode":     0,
		"OpMetaEvictInode":    0,
		"OpMetaSetattr":       0,
	}
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
	var opMap map[string]int
	key := fmt.Sprintf("%v_%v", vol, pid)
	if v, ok := cdb.VolOpCount.Load(key); ok {
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
	cdb.VolOpCount.Store(key, opMap)
}

func clearOpCount(opCountMap map[string]int) {
	for k := range opCountMap {
		opCountMap[k] = 0
	}
}

func (cdb *CdbStore) InsertCDB(nodeID uint64) {
	var (
		body []byte
		err  error
	)
	timestamp := time.Now().Unix()
	cdb.VolOpCount.Range(func(key, value interface{}) bool {
		id := fmt.Sprintf("%v_%v", key, timestamp)
		url := fmt.Sprintf("http://%v/put/%v/%v", cdb.Addr, cdb.Table, id)
		ops := value.(map[string]int)
		item := make(map[string]interface{})
		for k, v := range ops {
			item[k] = v
		}
		vol, pid := getVolAndPid(key.(string))
		item[FieldVol] = vol
		item[FieldPid] = pid
		item[FieldTime] = timestamp
		clearOpCount(ops)
		if body, err = json.Marshal(item); err != nil {
			log.LogErrorf("insert chubaodb: json marshal err[%v], data[%v]", err, body)
			return true
		}
		sendRequest(url, body)
		return true
	})
}

func getVolAndPid(key string) (vol, pid string) {
	index := strings.Index(key, "_")
	return key[:index], key[index+1:]
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
