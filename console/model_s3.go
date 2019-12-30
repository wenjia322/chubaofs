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
	"time"
	"net/http"
	"encoding/json"
)

const (
	CodeFailed  = "8888"
	CodeSuccess = "0000"
	ContentTypeHeaderName = "Content-Type"
	ContentTypeJsonValue = "application/json"
	ResponseSuccess = "Operation success"
)

type Bucket struct {
	Name       string
	Creator    string
	CreateTime time.Time
}

type RestResponse struct {
	Code    string
	Message string
	Data    interface{}
}

func writeSuccessResponse(w http.ResponseWriter) {
	er := &RestResponse{
		Code:    CodeSuccess,
		Message: ResponseSuccess,
	}

	data, _ := json.Marshal(er)
	w.WriteHeader(http.StatusOK)
	w.Header().Set(ContentTypeHeaderName, ContentTypeJsonValue)
	w.Write(data)
}

func writeDataResponse(w http.ResponseWriter, data interface{}) {
	rr := &RestResponse{
		Data:    data,
		Code:    CodeSuccess,
		Message: ResponseSuccess,
	}

	response, _ := json.Marshal(rr)
	w.WriteHeader(http.StatusOK)
	w.Header().Set(ContentTypeHeaderName, ContentTypeJsonValue)
	w.Write(response)
}

func writeErrorResponse(w http.ResponseWriter, message string) {
	er := &RestResponse{
		Code:    CodeFailed,
		Message: message,
	}

	data, _ := json.Marshal(er)
	w.WriteHeader(http.StatusOK)
	w.Header().Set(ContentTypeHeaderName, ContentTypeJsonValue)
	w.Write(data)
}
