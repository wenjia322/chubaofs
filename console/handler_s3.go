// Copyright 2018 The ChubaoFS Authors.
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

package console

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (c *Console) listBucketsHandler(w http.ResponseWriter, r *http.Request) {
	b1 := &Bucket{
		Name:       "pictures",
		Creator:    "liyubo4",
		CreateTime: time.Now(),
	}
	b2 := &Bucket{
		Name:       "testFile",
		Creator:    "liyubo4",
		CreateTime: time.Now(),
	}
	b3 := &Bucket{
		Name:       "pictures",
		Creator:    "liyubo4",
		CreateTime: time.Now(),
	}

	bs := make([]*Bucket, 0)
	bs = append(bs, b1)
	bs = append(bs, b2)
	bs = append(bs, b3)

	data, err := json.Marshal(bs)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(string(data))
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (c *Console) createBucketHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) deleteBucketHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) listObjectsHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) putObjectHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) getObjectHandler(w http.ResponseWriter, r *http.Request) {

}
