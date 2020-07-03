// Copyright 2019 The ChubaoFS Authors.
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

package objectnode

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/chubaofs/chubaofs/util/log"
	"github.com/gorilla/mux"
)

func (o *ObjectNode) getBucketPolicyHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err error
		ec  *ErrorCode
	)
	defer o.errorResponse(w, r, err, ec)

	var param = ParseRequestParam(r)
	if param.Bucket() == "" {
		ec = InvalidBucketName
		return
	}
	var vol *Volume
	if vol, err = o.getVol(param.Bucket()); err != nil {
		log.LogErrorf("getBucketPolicyHandler: load volume fail: requestID(%v) err(%v)",
			GetRequestID(r), err)
		ec = NoSuchBucket
		return
	}
	var policy *Policy
	if policy, err = vol.metaLoader.loadPolicy(); err != nil {
		ec = InternalErrorCode(err)
		return
	}

	var policyData []byte
	policyData, err = json.Marshal(policy)
	if err != nil {
		ec = InternalErrorCode(err)
		return
	}

	_, _ = w.Write(policyData)

	return
}

func (o *ObjectNode) putBucketPolicyHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err error
		ec  *ErrorCode
	)
	defer o.errorResponse(w, r, err, ec)

	var param = ParseRequestParam(r)
	if param.Bucket() == "" {
		ec = InvalidBucketName
		return
	}
	var vol *Volume
	if vol, err = o.getVol(param.Bucket()); err != nil {
		log.LogErrorf("putBucketPolicyHandler: load volume fail: requestID(%v) err(%v)",
			GetRequestID(r), err)
		ec = NoSuchBucket
		return
	}

	if r.ContentLength > BucketPolicyLimitSize {
		ec = MaxContentLength
		return
	}

	var bytes []byte
	bytes, err = ioutil.ReadAll(r.Body)
	if err != nil && err != io.EOF {
		log.LogErrorf("putBucketPolicyHandler: read request body fail: requestID(%v) err(%v)", GetRequestID(r), err)
		ec = &ErrorCode{
			ErrorCode:    http.StatusText(http.StatusBadRequest),
			ErrorMessage: err.Error(),
			StatusCode:   http.StatusBadRequest,
		}
		return
	}

	var policy *Policy
	policy, err = storeBucketPolicy(bytes, vol)
	if err != nil {
		log.LogErrorf("putBucketPolicyHandler: store policy fail: requestID(%v) err(%v)", GetRequestID(r), err)
		ec = InternalErrorCode(err)
		return
	}

	log.LogInfof("putBucketPolicyHandler: put bucket policy: requestID(%v) volume(%v) policy(%v)",
		GetRequestID(r), param.Bucket(), policy)

	return
}

func (o *ObjectNode) deleteBucketPolicyHandler(w http.ResponseWriter, r *http.Request) {
	log.LogInfof("delete bucket policy...")
	var (
		err error
		ec  *ErrorCode
	)
	defer o.errorResponse(w, r, err, ec)

	vars := mux.Vars(r)
	bucket := vars["bucket"]
	if bucket == "" {
		err = errors.New("")
		ec = NoSuchBucket
		return
	}
	// todo: implement
	return
}
