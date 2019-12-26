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
	"github.com/gorilla/mux"
	"net/http"
)

func (c *Console) registerApiRouters(router *mux.Router) {

	// S3 router for bucket
	router.Methods(http.MethodGet).Path("/bucket/list").HandlerFunc(c.listBucketsHandler)
	router.Methods(http.MethodGet).Path("/bucket/create").HandlerFunc(c.createBucketHandler)
	router.Methods(http.MethodGet).Path("/bucket/delete").HandlerFunc(c.deleteBucketHandler)

	// S3 router for object
	router.Methods(http.MethodGet).Path("/object/list").HandlerFunc(c.listObjectsHandler)
	router.Methods(http.MethodGet).Path("/object/put").HandlerFunc(c.putObjectHandler)
	router.Methods(http.MethodGet).Path("/object/get").HandlerFunc(c.getObjectHandler)

	// monitor router

}
