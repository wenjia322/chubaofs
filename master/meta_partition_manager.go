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

package master

import (
	"fmt"
	"github.com/chubaofs/chubaofs/proto"
	"github.com/chubaofs/chubaofs/util/log"
	"math"
	"strconv"
	"time"
)

func (c *Cluster) scheduleToLoadMetaPartitions() {
	go func() {
		for {
			if c.partition != nil && c.partition.IsRaftLeader() {
				if c.vols != nil {
					c.checkLoadMetaPartitions()
				}
			}
			time.Sleep(2 * time.Second * defaultIntervalToCheckDataPartition)
		}
	}()
}

func (c *Cluster) checkLoadMetaPartitions() {
	defer func() {
		if r := recover(); r != nil {
			log.LogWarnf("checkDiskRecoveryProgress occurred panic,err[%v]", r)
			WarnBySpecialKey(fmt.Sprintf("%v_%v_scheduling_job_panic", c.Name, ModuleName),
				"checkDiskRecoveryProgress occurred panic")
		}
	}()
	vols := c.allVols()
	for _, vol := range vols {
		mps := vol.cloneMetaPartitionMap()
		for _, mp := range mps {
			c.doLoadMetaPartition(mp)
		}
	}
}

func (mp *MetaPartition) checkSnapshot(clusterID string) {
	if len(mp.LoadResponse) == 0 {
		return
	}
	if !mp.doCompare() {
		return
	}
	if !mp.isSameApplyID() {
		return
	}
	mp.checkInodeCount(clusterID)
	mp.checkDentryCount(clusterID)
}

func (mp *MetaPartition) doCompare() bool {
	for _, lr := range mp.LoadResponse {
		if !lr.DoCompare {
			return false
		}
	}
	return true
}

func (mp *MetaPartition) isSameApplyID() bool {
	rst := true
	applyID := mp.LoadResponse[0].ApplyID
	for _, loadResponse := range mp.LoadResponse {
		if applyID != loadResponse.ApplyID {
			rst = false
		}
	}
	return rst
}

func (mp *MetaPartition) checkInodeCount(clusterID string) {
	isEqual := true
	maxInode := mp.LoadResponse[0].MaxInode
	for _, loadResponse := range mp.LoadResponse {
		diff := math.Abs(float64(loadResponse.MaxInode) - float64(maxInode))
		if diff > defaultRangeOfCountDifferencesAllowed {
			isEqual = false
		}
	}

	if !isEqual {
		msg := fmt.Sprintf("inode count is not equal,vol[%v],mpID[%v],", mp.volName, mp.PartitionID)
		for _, lr := range mp.LoadResponse {
			inodeCountStr := strconv.FormatUint(lr.MaxInode, 10)
			applyIDStr := strconv.FormatUint(uint64(lr.ApplyID), 10)
			msg = msg + lr.Addr + " applyId[" + applyIDStr + "] maxInode[" + inodeCountStr + "],"
		}
		Warn(clusterID, msg)
	}
}

func (mp *MetaPartition) checkDentryCount(clusterID string) {
	isEqual := true
	dentryCount := mp.LoadResponse[0].DentryCount
	for _, loadResponse := range mp.LoadResponse {
		diff := math.Abs(float64(loadResponse.DentryCount) - float64(dentryCount))
		if diff > defaultRangeOfCountDifferencesAllowed {
			isEqual = false
		}
	}

	if !isEqual {
		msg := fmt.Sprintf("dentry count is not equal,vol[%v],mpID[%v],", mp.volName, mp.PartitionID)
		for _, lr := range mp.LoadResponse {
			dentryCountStr := strconv.FormatUint(lr.DentryCount, 10)
			applyIDStr := strconv.FormatUint(uint64(lr.ApplyID), 10)
			msg = msg + lr.Addr + " applyId[" + applyIDStr + "] dentryCount[" + dentryCountStr + "],"
		}
		Warn(clusterID, msg)
	}
}

func (c *Cluster) scheduleToCheckMetaPartitionRecoveryProgress() {
	go func() {
		for {
			if c.partition != nil && c.partition.IsRaftLeader() {
				if c.vols != nil {
					c.checkMetaPartitionRecoveryProgress()
				}
			}
			time.Sleep(time.Second * defaultIntervalToCheckDataPartition)
		}
	}()
}

func (c *Cluster) checkMetaPartitionRecoveryProgress() {
	defer func() {
		if r := recover(); r != nil {
			log.LogWarnf("checkMetaPartitionRecoveryProgress occurred panic,err[%v]", r)
			WarnBySpecialKey(fmt.Sprintf("%v_%v_scheduling_job_panic", c.Name, ModuleName),
				"checkMetaPartitionRecoveryProgress occurred panic")
		}
	}()

	var diff float64
	c.fullFillMpReplica()
	c.BadMetaPartitionIds.Range(func(key, value interface{}) bool {
		badMetaPartitionIds := value.([]uint64)
		newBadMpIds := make([]uint64, 0)
		for _, partitionID := range badMetaPartitionIds {
			partition, err := c.getMetaPartitionByID(partitionID)
			if err != nil {
				continue
			}
			vol, err := c.getVol(partition.volName)
			if err != nil {
				continue
			}
			if len(partition.Replicas) == 0 {
				continue
			}
			diff = partition.getMinusOfMaxInodeID()
			if diff < defaultMinusOfMaxInodeID && int(vol.mpReplicaNum) <= len(partition.Replicas){
				partition.IsRecover = false
				partition.RLock()
				c.syncUpdateMetaPartition(partition)
				partition.RUnlock()
				Warn(c.Name, fmt.Sprintf("clusterID[%v],vol[%v] partitionID[%v] has recovered success", c.Name, partition.volName, partitionID))
			} else {
				newBadMpIds = append(newBadMpIds, partitionID)
			}
		}

		if len(newBadMpIds) == 0 {
			Warn(c.Name, fmt.Sprintf("clusterID[%v],node[%v] has recovered success", c.Name, key))
			c.BadMetaPartitionIds.Delete(key)
		} else {
			c.BadMetaPartitionIds.Store(key, newBadMpIds)
		}

		return true
	})
}
// Add replica for the partition whose replica number is less than replicaNum
func (c *Cluster) fullFillMpReplica() {
	c.BadMetaPartitionIds.Range(func(key, value interface{}) bool {
		badMetaPartitionIds := value.([]uint64)
		badAddr := key.(string)
		newBadParitionIds := make([]uint64, 0)
		for _, partitionID := range badMetaPartitionIds {
			var isSkip bool
			var err    error
			if isSkip, err = c.checkAddMpReplica(badAddr, partitionID); err != nil {
				log.LogWarnf(fmt.Sprintf("action[fullFillReplica], clusterID[%v], partitionID[%v], err[%v] ", c.Name, partitionID, err))
			}
			if !isSkip {
				newBadParitionIds = append(newBadParitionIds, partitionID)
			}
		}
		//Todo: write BadMetaPartitionIds to raft log
		c.BadMetaPartitionIds.Store(key, newBadParitionIds)
		return true
	})

}

func (c *Cluster) checkAddMpReplica(badAddr string, partitionID uint64) (isSkip bool, err error){
	var(
		newPeer    proto.Peer
		partition  *MetaPartition
	)
	if partition, err = c.getMetaPartitionByID(partitionID); err != nil {
		return
	}
	if int(partition.ReplicaNum) == len(partition.Replicas) {
		return
	}
	if _, err = partition.getMetaReplicaLeader(); err != nil {
		log.LogWarnf(fmt.Sprintf("Action[checkAddMpReplica], partitionID[%v], no leader", partitionID))
		return
	}
	if newPeer, err = c.chooseTargetMetaPartitionHost(badAddr, partition); err != nil {
		return
	}
	if err = c.addMetaReplica(partition, newPeer.Addr); err != nil {
		return
	}
	// Todo: What if Master changed leader before this step?
	if int(partition.ReplicaNum) > len(partition.Replicas) {
		isSkip = true
	}
	return
}
