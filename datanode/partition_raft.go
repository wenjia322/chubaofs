// Copyright 2018 The Chubao Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package datanode

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/chubaofs/chubaofs/proto"
	"github.com/chubaofs/chubaofs/raftstore"
	"github.com/chubaofs/chubaofs/repl"
	"github.com/chubaofs/chubaofs/util/config"
	"github.com/chubaofs/chubaofs/util/errors"
	"github.com/chubaofs/chubaofs/util/log"
	raftproto "github.com/tiglabs/raft/proto"
)

type dataPartitionCfg struct {
	VolName       string              `json:"vol_name"`
	ClusterID     string              `json:"cluster_id"`
	PartitionID   uint64              `json:"partition_id"`
	PartitionSize int                 `json:"partition_size"`
	Peers         []proto.Peer        `json:"peers"`
	Hosts         []string            `json:"hosts"`
	Learners      []proto.Learner     `json:"learners"`
	NodeID        uint64              `json:"-"`
	RaftStore     raftstore.RaftStore `json:"-"`
}

func (dp *DataPartition) raftPort() (heartbeat, replica int, err error) {
	raftConfig := dp.config.RaftStore.RaftConfig()
	heartbeatAddrSplits := strings.Split(raftConfig.HeartbeatAddr, ":")
	replicaAddrSplits := strings.Split(raftConfig.ReplicateAddr, ":")
	if len(heartbeatAddrSplits) != 2 {
		err = errors.New("illegal heartbeat address")
		return
	}
	if len(replicaAddrSplits) != 2 {
		err = errors.New("illegal replica address")
		return
	}
	heartbeat, err = strconv.Atoi(heartbeatAddrSplits[1])
	if err != nil {
		return
	}
	replica, err = strconv.Atoi(replicaAddrSplits[1])
	if err != nil {
		return
	}
	return
}

// StartRaft start raft instance when data partition start or restore.
func (dp *DataPartition) StartRaft() (err error) {
	var (
		heartbeatPort int
		replicaPort   int
		peers         []raftstore.PeerAddress
		learners      []raftproto.Learner
	)
	defer func() {
		if r := recover(); r != nil {
			mesg := fmt.Sprintf("StartRaft(%v)  Raft Panic (%v)", dp.partitionID, r)
			panic(mesg)
		}
	}()

	if heartbeatPort, replicaPort, err = dp.raftPort(); err != nil {
		return
	}
	for _, peer := range dp.config.Peers {
		addr := strings.Split(peer.Addr, ":")[0]
		rp := raftstore.PeerAddress{
			Peer: raftproto.Peer{
				ID: peer.ID,
			},
			Address:       addr,
			HeartbeatPort: heartbeatPort,
			ReplicaPort:   replicaPort,
		}
		peers = append(peers, rp)
	}
	for _, learner := range dp.config.Learners {
		addLearner := raftproto.Learner{
			ID:         learner.ID,
			PromConfig: &raftproto.PromoteConfig{AutoPromote: learner.PmConfig.AutoProm, PromThreshold: learner.PmConfig.PromThreshold},
		}
		learners = append(learners, addLearner)
	}
	log.LogDebugf("start partition(%v) raft peers: %s path: %s",
		dp.partitionID, peers, dp.path)
	pc := &raftstore.PartitionConfig{
		ID:       uint64(dp.partitionID),
		Applied:  dp.appliedID,
		Peers:    peers,
		Learners: learners,
		SM:       dp,
		WalPath:  dp.path,
	}

	dp.raftPartition, err = dp.config.RaftStore.CreatePartition(pc)
	if err == nil {
		dp.ForceSetDataPartitionToFininshLoad()
	}
	return
}

func (dp *DataPartition) stopRaft() {
	if dp.raftPartition != nil {
		log.LogErrorf("[FATAL] stop raft partition(%v)", dp.partitionID)
		dp.raftPartition.Stop()
		dp.raftPartition = nil
	}
	return
}

func (dp *DataPartition) CanRemoveRaftMember(peer proto.Peer) error {
	for _, learner := range dp.config.Learners {
		if learner.ID == peer.ID && learner.Addr == peer.Addr {
			return nil
		}
	}
	downReplicas := dp.config.RaftStore.RaftServer().GetDownReplicas(dp.partitionID)
	hasExsit := false
	for _, p := range dp.config.Peers {
		if p.ID == peer.ID {
			hasExsit = true
			break
		}
	}
	if !hasExsit {
		return nil
	}

	hasDownReplicasExcludePeer := make([]uint64, 0)
	for _, nodeID := range downReplicas {
		if nodeID.NodeID == peer.ID {
			continue
		}
		hasDownReplicasExcludePeer = append(hasDownReplicasExcludePeer, nodeID.NodeID)
	}

	sumReplicas := len(dp.config.Peers) - len(dp.config.Learners)
	if sumReplicas%2 == 1 {
		if sumReplicas-len(hasDownReplicasExcludePeer) > (sumReplicas/2 + 1) {
			return nil
		}
	} else {
		if sumReplicas-len(hasDownReplicasExcludePeer) >= (sumReplicas/2 + 1) {
			return nil
		}
	}

	return fmt.Errorf("hasDownReplicasExcludePeer(%v) too much,so donnot offline (%v)", downReplicas, peer)
}

// StartRaftLoggingSchedule starts the task schedule as follows:
// 1. write the raft applied id into disk.
// 2. collect the applied ids from raft members.
// 3. based on the minimum applied id to cutoff and delete the saved raft log in order to free the disk space.
func (dp *DataPartition) StartRaftLoggingSchedule() {
	getAppliedIDTimer := time.NewTimer(time.Second * 1)
	truncateRaftLogTimer := time.NewTimer(time.Minute * 10)
	storeAppliedIDTimer := time.NewTimer(time.Second * 10)

	log.LogDebugf("[startSchedule] hello DataPartition schedule")

	for {
		select {
		case <-dp.stopC:
			log.LogDebugf("[startSchedule] stop partition(%v)", dp.partitionID)
			getAppliedIDTimer.Stop()
			truncateRaftLogTimer.Stop()
			storeAppliedIDTimer.Stop()
			return

		case extentID := <-dp.stopRaftC:
			dp.stopRaft()
			log.LogErrorf("action[ExtentRepair] stop raft partition(%v)_%v", dp.partitionID, extentID)

		case <-getAppliedIDTimer.C:
			if dp.raftPartition != nil {
				dp.updateMaxMinAppliedID()
			}
			getAppliedIDTimer.Reset(time.Minute * 1)

		case <-truncateRaftLogTimer.C:
			if dp.raftPartition == nil {
				break
			}

			if dp.minAppliedID > dp.lastTruncateID { // Has changed
				dp.raftPartition.Truncate(dp.minAppliedID)
				dp.lastTruncateID = dp.minAppliedID
				dp.PersistMetadata()
				log.LogInfof("PartitionID(%v) truncated RaftLog to (%v)", dp.partitionID, dp.minAppliedID)
			}
			truncateRaftLogTimer.Reset(time.Minute)

		case <-storeAppliedIDTimer.C:
			if err := dp.storeAppliedID(dp.appliedID); err != nil {
				err = errors.NewErrorf("[startSchedule]: dump partition=%d: %v", dp.config.PartitionID, err.Error())
				log.LogErrorf(err.Error())
			}
			storeAppliedIDTimer.Reset(time.Second * 10)
		}
	}
}

// StartRaftAfterRepair starts the raft after repairing a partition.
// It can only happens after all the extent files are repaired by the leader.
// When the repair is finished, the local dp.partitionSize is same as the leader's dp.partitionSize.
// The repair task can be done in statusUpdateScheduler->LaunchRepair.
func (dp *DataPartition) StartRaftAfterRepair() {
	var (
		initPartitionSize, initMaxExtentID uint64
		currLeaderPartitionSize            uint64
		err                                error
	)
	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			err = nil
			if dp.isLeader { // primary does not need to wait repair
				if err := dp.StartRaft(); err != nil {
					log.LogErrorf("PartitionID(%v) leader start raft err(%v).", dp.partitionID, err)
					timer.Reset(5 * time.Second)
					continue
				}
				log.LogDebugf("PartitionID(%v) leader started.", dp.partitionID)
				return
			}

			// wait for dp.replicas to be updated
			if dp.getReplicaLen() == 0 {
				timer.Reset(5 * time.Second)
				continue
			}
			if initMaxExtentID == 0 || initPartitionSize == 0 {
				initMaxExtentID, initPartitionSize, err = dp.getLeaderMaxExtentIDAndPartitionSize()
			}

			if err != nil {
				log.LogErrorf("PartitionID(%v) get MaxExtentID  err(%v)", dp.partitionID, err)
				timer.Reset(5 * time.Second)
				continue
			}

			// get the partition size from the primary and compare it with the loparal one
			currLeaderPartitionSize, err = dp.getLeaderPartitionSize(initMaxExtentID)
			if err != nil {
				log.LogErrorf("PartitionID(%v) get leader size err(%v)", dp.partitionID, err)
				timer.Reset(5 * time.Second)
				continue
			}

			if currLeaderPartitionSize < initPartitionSize {
				initPartitionSize = currLeaderPartitionSize
			}
			localSize := dp.extentStore.StoreSizeExtentID(initMaxExtentID)

			log.LogInfof("StartRaftAfterRepair PartitionID(%v) initMaxExtentID(%v) initPartitionSize(%v) currLeaderPartitionSize(%v)"+
				"localSize(%v)", dp.partitionID, initMaxExtentID, initPartitionSize, currLeaderPartitionSize, localSize)

			if initPartitionSize > localSize {
				log.LogErrorf("PartitionID(%v) leader size(%v) local size(%v) wait snapshot recover", dp.partitionID, initPartitionSize, localSize)
				timer.Reset(5 * time.Second)
				continue
			}

			// start raft
			dp.DataPartitionCreateType = proto.NormalCreateDataPartition
			dp.PersistMetadata()
			if err := dp.StartRaft(); err != nil {
				log.LogErrorf("PartitionID(%v) start raft err(%v). Retry after 20s.", dp.partitionID, err)
				timer.Reset(5 * time.Second)
				continue
			}
			log.LogInfof("PartitionID(%v) raft started.", dp.partitionID)
			return
		case <-dp.stopC:
			timer.Stop()
			return
		}
	}
}

// Add a raft node.
func (dp *DataPartition) addRaftNode(req *proto.AddDataPartitionRaftMemberRequest, index uint64) (isUpdated bool, err error) {
	var (
		heartbeatPort int
		replicaPort   int
	)
	if heartbeatPort, replicaPort, err = dp.raftPort(); err != nil {
		return
	}

	found := false
	for _, peer := range dp.config.Peers {
		if peer.ID == req.AddPeer.ID {
			found = true
			break
		}
	}
	isUpdated = !found
	if !isUpdated {
		return
	}
	data, _ := json.Marshal(req)
	log.LogInfof("addRaftNode: remove self: partitionID(%v) nodeID(%v) index(%v) data(%v) ",
		req.PartitionId, dp.config.NodeID, index, string(data))
	dp.config.Peers = append(dp.config.Peers, req.AddPeer)
	dp.config.Hosts = append(dp.config.Hosts, req.AddPeer.Addr)
	dp.replicasLock.Lock()
	dp.replicas = make([]string, len(dp.config.Hosts))
	copy(dp.replicas, dp.config.Hosts)
	dp.replicasLock.Unlock()
	addr := strings.Split(req.AddPeer.Addr, ":")[0]
	dp.config.RaftStore.AddNodeWithPort(req.AddPeer.ID, addr, heartbeatPort, replicaPort)
	return
}

// Delete a raft node.
func (dp *DataPartition) removeRaftNode(req *proto.RemoveDataPartitionRaftMemberRequest, index uint64) (isUpdated bool, err error) {
	var canRemoveSelf bool
	if canRemoveSelf, err = dp.canRemoveSelf(); err != nil {
		return
	}
	peerIndex := -1
	data, _ := json.Marshal(req)
	isUpdated = false
	log.LogInfof("Start RemoveRaftNode  PartitionID(%v) nodeID(%v)  do RaftLog (%v) ",
		req.PartitionId, dp.config.NodeID, string(data))
	for i, peer := range dp.config.Peers {
		if peer.ID == req.RemovePeer.ID {
			peerIndex = i
			isUpdated = true
			break
		}
	}
	if !isUpdated {
		log.LogInfof("NoUpdate RemoveRaftNode  PartitionID(%v) nodeID(%v)  do RaftLog (%v) ",
			req.PartitionId, dp.config.NodeID, string(data))
		return
	}
	hostIndex := -1
	for index, host := range dp.config.Hosts {
		if host == req.RemovePeer.Addr {
			hostIndex = index
			break
		}
	}
	if hostIndex != -1 {
		dp.config.Hosts = append(dp.config.Hosts[:hostIndex], dp.config.Hosts[hostIndex+1:]...)
	}
	dp.config.Peers = append(dp.config.Peers[:peerIndex], dp.config.Peers[peerIndex+1:]...)
	learnerIndex := -1
	for i, learner := range dp.config.Learners {
		if learner.ID == req.RemovePeer.ID && learner.Addr == req.RemovePeer.Addr {
			learnerIndex = i
			break
		}
	}
	if learnerIndex != -1 {
		dp.config.Learners = append(dp.config.Learners[:learnerIndex], dp.config.Learners[learnerIndex+1:]...)
	}
	if dp.config.NodeID == req.RemovePeer.ID && !dp.isLoadingDataPartition && canRemoveSelf {
		dp.raftPartition.Delete()
		dp.Disk().space.DeletePartition(dp.partitionID)
		isUpdated = false
	}
	log.LogInfof("Fininsh RemoveRaftNode  PartitionID(%v) nodeID(%v)  do RaftLog (%v) ",
		req.PartitionId, dp.config.NodeID, string(data))

	return
}

// Add a raft learner.
func (dp *DataPartition) addRaftLearner(req *proto.AddDataPartitionRaftLearnerRequest, index uint64) (isUpdated bool, err error) {
	var (
		heartbeatPort int
		replicaPort   int
	)
	if heartbeatPort, replicaPort, err = dp.raftPort(); err != nil {
		return
	}

	addPeer := false
	for _, peer := range dp.config.Peers {
		if peer.ID == req.AddLearner.ID {
			addPeer = true
			break
		}
	}
	if !addPeer {
		peer := proto.Peer{ID: req.AddLearner.ID, Addr: req.AddLearner.Addr}
		dp.config.Peers = append(dp.config.Peers, peer)
		dp.config.Hosts = append(dp.config.Hosts, peer.Addr)
	}

	addLearner := false
	for _, learner := range dp.config.Learners {
		if learner.ID == req.AddLearner.ID {
			addLearner = true
			break
		}
	}
	if !addLearner {
		dp.config.Learners = append(dp.config.Learners, req.AddLearner)
	}
	isUpdated = !addPeer || !addLearner
	if !isUpdated {
		return
	}
	log.LogInfof("addRaftLearner: partitionID(%v) nodeID(%v) index(%v) data(%v) ",
		req.PartitionId, dp.config.NodeID, index, req)
	dp.replicasLock.Lock()
	dp.replicas = make([]string, len(dp.config.Hosts))
	copy(dp.replicas, dp.config.Hosts)
	dp.replicasLock.Unlock()
	addr := strings.Split(req.AddLearner.Addr, ":")[0]
	dp.config.RaftStore.AddNodeWithPort(req.AddLearner.ID, addr, heartbeatPort, replicaPort)
	return
}

// Promote a raft learner.
func (dp *DataPartition) promoteRaftLearner(req *proto.PromoteDataPartitionRaftLearnerRequest, index uint64) (isUpdated bool, err error) {
	var promoteIndex int
	for i, learner := range dp.config.Learners {
		if learner.ID == req.PromoteLearner.ID {
			isUpdated = true
			promoteIndex = i
			break
		}
	}
	if isUpdated {
		dp.config.Learners = append(dp.config.Learners[:promoteIndex], dp.config.Learners[promoteIndex+1:]...)
		log.LogInfof("promoteRaftLearner: partitionID(%v) nodeID(%v) index(%v) data(%v), new learners(%v) ",
			req.PartitionId, dp.config.NodeID, index, req, dp.config.Learners)
	}
	return
}

func (dp *DataPartition) storeAppliedID(applyIndex uint64) (err error) {
	filename := path.Join(dp.Path(), TempApplyIndexFile)
	fp, err := os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		return
	}
	defer func() {
		fp.Close()
		os.Remove(filename)
	}()
	if _, err = fp.WriteString(fmt.Sprintf("%d", applyIndex)); err != nil {
		return
	}
	fp.Sync()
	err = os.Rename(filename, path.Join(dp.Path(), ApplyIndexFile))
	return
}

// LoadAppliedID loads the applied IDs to the memory.
func (dp *DataPartition) LoadAppliedID() (err error) {
	filename := path.Join(dp.Path(), ApplyIndexFile)
	if _, err = os.Stat(filename); err != nil {
		err = nil
		return
	}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if err == os.ErrNotExist {
			err = nil
			return
		}
		err = errors.NewErrorf("[loadApplyIndex] OpenFile: %s", err.Error())
		return
	}
	if len(data) == 0 {
		err = errors.NewErrorf("[loadApplyIndex]: ApplyIndex is empty")
		return
	}
	if _, err = fmt.Sscanf(string(data), "%d", &dp.appliedID); err != nil {
		err = errors.NewErrorf("[loadApplyID] ReadApplyID: %s", err.Error())
		return
	}
	return
}

func (dp *DataPartition) SetMinAppliedID(id uint64) {
	dp.minAppliedID = id
}

func (dp *DataPartition) GetAppliedID() (id uint64) {
	return dp.appliedID
}

func (s *DataNode) parseRaftConfig(cfg *config.Config) (err error) {
	s.raftDir = cfg.GetString(ConfigKeyRaftDir)
	if s.raftDir == "" {
		return fmt.Errorf("bad raftDir config")
	}
	s.raftHeartbeat = cfg.GetString(ConfigKeyRaftHeartbeat)
	s.raftReplica = cfg.GetString(ConfigKeyRaftReplica)
	log.LogDebugf("[parseRaftConfig] load raftDir(%v).", s.raftDir)
	log.LogDebugf("[parseRaftConfig] load raftHearbeat(%v).", s.raftHeartbeat)
	log.LogDebugf("[parseRaftConfig] load raftReplica(%v).", s.raftReplica)
	return
}

func (s *DataNode) startRaftServer(cfg *config.Config) (err error) {
	log.LogInfo("Start: startRaftServer")

	s.parseRaftConfig(cfg)

	constCfg := config.ConstConfig{
		Listen:           s.port,
		RaftHeartbetPort: s.raftHeartbeat,
		RaftReplicaPort:  s.raftReplica,
	}
	var ok = false
	if ok, err = config.CheckOrStoreConstCfg(s.raftDir, config.DefaultConstConfigFile, &constCfg); !ok {
		log.LogErrorf("constCfg check failed %v %v %v %v", s.raftDir, config.DefaultConstConfigFile, constCfg, err)
		return fmt.Errorf("constCfg check failed %v %v %v %v", s.raftDir, config.DefaultConstConfigFile, constCfg, err)
	}

	if _, err = os.Stat(s.raftDir); err != nil {
		if err = os.MkdirAll(s.raftDir, 0755); err != nil {
			err = errors.NewErrorf("create raft server dir: %s", err.Error())
			log.LogErrorf("action[startRaftServer] cannot start raft server err(%v)", err)
			return
		}
	}

	heartbeatPort, err := strconv.Atoi(s.raftHeartbeat)
	if err != nil {
		err = errors.NewErrorf("Raft heartbeat port configuration error: %s", err.Error())
		return
	}
	replicatePort, err := strconv.Atoi(s.raftReplica)
	if err != nil {
		err = errors.NewErrorf("Raft replica port configuration error: %s", err.Error())
		return
	}

	raftConf := &raftstore.Config{
		NodeID:            s.nodeID,
		RaftPath:          s.raftDir,
		IPAddr:            LocalIP,
		HeartbeatPort:     heartbeatPort,
		ReplicaPort:       replicatePort,
		NumOfLogsToRetain: DefaultRaftLogsToRetain,
	}
	s.raftStore, err = raftstore.NewRaftStore(raftConf)
	if err != nil {
		err = errors.NewErrorf("new raftStore: %s", err.Error())
		log.LogErrorf("action[startRaftServer] cannot start raft server err(%v)", err)
	}

	return
}

func (s *DataNode) stopRaftServer() {
	if s.raftStore != nil {
		s.raftStore.Stop()
	}
}

// NewPacketToBroadcastMinAppliedID returns a new packet to broadcast the min applied ID.
func NewPacketToBroadcastMinAppliedID(partitionID uint64, minAppliedID uint64) (p *repl.Packet) {
	p = new(repl.Packet)
	p.Opcode = proto.OpBroadcastMinAppliedID
	p.PartitionID = partitionID
	p.Magic = proto.ProtoMagic
	p.ReqID = proto.GenerateRequestID()
	p.Data = make([]byte, 8)
	binary.BigEndian.PutUint64(p.Data, minAppliedID)
	p.Size = uint32(len(p.Data))
	return
}

// NewPacketToGetAppliedID returns a new packet to get the applied ID.
func NewPacketToGetAppliedID(partitionID uint64) (p *repl.Packet) {
	p = new(repl.Packet)
	p.Opcode = proto.OpGetAppliedId
	p.PartitionID = partitionID
	p.Magic = proto.ProtoMagic
	p.ReqID = proto.GenerateRequestID()
	return
}

// NewPacketToGetPartitionSize returns a new packet to get the partition size.
func NewPacketToGetPartitionSize(partitionID uint64) (p *repl.Packet) {
	p = new(repl.Packet)
	p.Opcode = proto.OpGetPartitionSize
	p.PartitionID = partitionID
	p.Magic = proto.ProtoMagic
	p.ReqID = proto.GenerateRequestID()
	return
}

// NewPacketToGetPartitionSize returns a new packet to get the partition size.
func NewPacketToGetMaxExtentIDAndPartitionSIze(partitionID uint64) (p *repl.Packet) {
	p = new(repl.Packet)
	p.Opcode = proto.OpGetMaxExtentIDAndPartitionSize
	p.PartitionID = partitionID
	p.Magic = proto.ProtoMagic
	p.ReqID = proto.GenerateRequestID()
	return
}

func (dp *DataPartition) findMinAppliedID(allAppliedIDs []uint64) (minAppliedID uint64, index int) {
	index = 0
	minAppliedID = allAppliedIDs[0]
	for i := 1; i < len(allAppliedIDs); i++ {
		if allAppliedIDs[i] < minAppliedID {
			minAppliedID = allAppliedIDs[i]
			index = i
		}
	}
	return minAppliedID, index
}

func (dp *DataPartition) findMaxAppliedID(allAppliedIDs []uint64) (maxAppliedID uint64, index int) {
	for i := 0; i < len(allAppliedIDs); i++ {
		if allAppliedIDs[i] > maxAppliedID {
			maxAppliedID = allAppliedIDs[i]
			index = i
		}
	}
	return maxAppliedID, index
}

// Get the partition size from the leader.
func (dp *DataPartition) getLeaderPartitionSize(maxExtentID uint64) (size uint64, err error) {
	var (
		conn *net.TCPConn
	)

	p := NewPacketToGetPartitionSize(dp.partitionID)
	p.ExtentID = maxExtentID
	target := dp.getReplicaAddr(0)
	conn, err = gConnPool.GetConnect(target) //get remote connect
	if err != nil {
		err = errors.Trace(err, " partition(%v) get host(%v) connect", dp.partitionID, target)
		return
	}
	defer gConnPool.PutConnect(conn, true)
	err = p.WriteToConn(conn) // write command to the remote host
	if err != nil {
		err = errors.Trace(err, "partition(%v) write to host(%v)", dp.partitionID, target)
		return
	}
	err = p.ReadFromConn(conn, 60)
	if err != nil {
		err = errors.Trace(err, "partition(%v) read from host(%v)", dp.partitionID, target)
		return
	}

	if p.ResultCode != proto.OpOk {
		err = errors.Trace(err, "partition(%v) result code not ok (%v) from host(%v)", dp.partitionID, p.ResultCode, target)
		return
	}
	size = binary.BigEndian.Uint64(p.Data)
	log.LogInfof("partition(%v) MaxExtentID(%v) size(%v)", dp.partitionID, maxExtentID, size)

	return
}

// Get the MaxExtentID partition  from the leader.
func (dp *DataPartition) getLeaderMaxExtentIDAndPartitionSize() (maxExtentID, PartitionSize uint64, err error) {
	var (
		conn *net.TCPConn
	)

	p := NewPacketToGetMaxExtentIDAndPartitionSIze(dp.partitionID)

	target := dp.getReplicaAddr(0)
	conn, err = gConnPool.GetConnect(target) //get remote connect
	if err != nil {
		err = errors.Trace(err, " partition(%v) get host(%v) connect", dp.partitionID, target)
		return
	}
	defer gConnPool.PutConnect(conn, true)
	err = p.WriteToConn(conn) // write command to the remote host
	if err != nil {
		err = errors.Trace(err, "partition(%v) write to host(%v)", dp.partitionID, target)
		return
	}
	err = p.ReadFromConn(conn, 60)
	if err != nil {
		err = errors.Trace(err, "partition(%v) read from host(%v)", dp.partitionID, target)
		return
	}

	if p.ResultCode != proto.OpOk {
		err = errors.Trace(err, "partition(%v) result code not ok (%v) from host(%v)", dp.partitionID, p.ResultCode, target)
		return
	}
	maxExtentID = binary.BigEndian.Uint64(p.Data[0:8])
	PartitionSize = binary.BigEndian.Uint64(p.Data[8:16])

	log.LogInfof("partition(%v) maxExtentID(%v) PartitionSize(%v) on leader", dp.partitionID, maxExtentID, PartitionSize)

	return
}

func (dp *DataPartition) broadcastMinAppliedID(minAppliedID uint64) (err error) {
	for i := 0; i < dp.getReplicaLen(); i++ {
		p := NewPacketToBroadcastMinAppliedID(dp.partitionID, minAppliedID)
		replicaHostParts := strings.Split(dp.getReplicaAddr(i), ":")
		replicaHost := strings.TrimSpace(replicaHostParts[0])
		if LocalIP == replicaHost {
			log.LogDebugf("partition(%v) local no send msg. localIP(%v) replicaHost(%v) appliedId(%v)",
				dp.partitionID, LocalIP, replicaHost, dp.appliedID)
			dp.minAppliedID = minAppliedID
			continue
		}
		target := dp.getReplicaAddr(i)
		var conn *net.TCPConn
		conn, err = gConnPool.GetConnect(target)
		if err != nil {
			return
		}
		defer gConnPool.PutConnect(conn, true)
		err = p.WriteToConn(conn)
		if err != nil {
			return
		}
		err = p.ReadFromConn(conn, 60)
		if err != nil {
			return
		}
		gConnPool.PutConnect(conn, true)

		log.LogDebugf("partition(%v) minAppliedID(%v)", dp.partitionID, minAppliedID)
	}

	return
}

// Get all replica applied ids
func (dp *DataPartition) getAllReplicaAppliedID() (allAppliedID []uint64, replyNum uint8) {
	allAppliedID = make([]uint64, dp.getReplicaLen())
	for i := 0; i < dp.getReplicaLen(); i++ {
		p := NewPacketToGetAppliedID(dp.partitionID)
		replicaHostParts := strings.Split(dp.getReplicaAddr(i), ":")
		replicaHost := strings.TrimSpace(replicaHostParts[0])
		if LocalIP == replicaHost {
			log.LogDebugf("partition(%v) local no send msg. localIP(%v) replicaHost(%v) appliedId(%v)",
				dp.partitionID, LocalIP, replicaHost, dp.appliedID)
			allAppliedID[i] = dp.appliedID
			replyNum++
			continue
		}
		target := dp.getReplicaAddr(i)
		appliedID, err := dp.getRemoteAppliedID(target, p)
		if err != nil {
			log.LogErrorf("partition(%v) getRemoteAppliedID Failed(%v).", dp.partitionID, err)
			continue
		}
		if appliedID == 0 {
			log.LogDebugf("[getAllReplicaAppliedID] partition(%v) local appliedID(%v) replicaHost(%v) appliedID=0",
				dp.partitionID, dp.appliedID, replicaHost)
		}
		allAppliedID[i] = appliedID
		replyNum++
	}

	return
}

// Get target members' applied id
func (dp *DataPartition) getRemoteAppliedID(target string, p *repl.Packet) (appliedID uint64, err error) {
	var conn *net.TCPConn
	start := time.Now().UnixNano()
	defer func() {
		if err != nil {
			err = fmt.Errorf(p.LogMessage(p.GetOpMsg(), target, start, err))
			log.LogErrorf(err.Error())
		}
	}()

	conn, err = gConnPool.GetConnect(target)
	if err != nil {
		return
	}
	defer gConnPool.PutConnect(conn, true)
	err = p.WriteToConn(conn) // write command to the remote host
	if err != nil {
		return
	}
	err = p.ReadFromConn(conn, 60)
	if err != nil {
		return
	}
	if p.ResultCode != proto.OpOk {
		err = errors.NewErrorf("partition(%v) result code not ok (%v) from host(%v)", dp.partitionID, p.ResultCode, target)
		return
	}
	appliedID = binary.BigEndian.Uint64(p.Data)

	log.LogDebugf("[getRemoteAppliedID] partition(%v) remoteAppliedID(%v)", dp.partitionID, appliedID)

	return
}

// Get all members' applied ids and find the minimum one
func (dp *DataPartition) updateMaxMinAppliedID() {
	var (
		minAppliedID uint64
		maxAppliedID uint64
	)

	// Get the applied id by the leader
	_, isLeader := dp.IsRaftLeader()
	if !isLeader {
		return
	}

	// if leader has not applied the raft, no need to get others
	if dp.appliedID == 0 {
		return
	}

	allAppliedID, replyNum := dp.getAllReplicaAppliedID()
	if replyNum == 0 {
		log.LogDebugf("[updateMaxMinAppliedID] PartitionID(%v) Get appliedId failed!", dp.partitionID)
		return
	}
	if replyNum == uint8(len(allAppliedID)) { // update dp.minAppliedID when every member had replied
		minAppliedID, _ = dp.findMinAppliedID(allAppliedID)
		log.LogDebugf("[updateMaxMinAppliedID] PartitionID(%v) localID(%v) OK! oldMinID(%v) newMinID(%v) allAppliedID(%v)",
			dp.partitionID, dp.appliedID, dp.minAppliedID, minAppliedID, allAppliedID)
		dp.broadcastMinAppliedID(minAppliedID)
	}

	maxAppliedID, _ = dp.findMaxAppliedID(allAppliedID)
	log.LogDebugf("[updateMaxMinAppliedID] PartitionID(%v) localID(%v) OK! oldMaxID(%v) newMaxID(%v)",
		dp.partitionID, dp.appliedID, dp.maxAppliedID, maxAppliedID)
	dp.maxAppliedID = maxAppliedID

	return
}
