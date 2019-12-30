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
	router.Methods(http.MethodPost).Path("/bucket/list").HandlerFunc(c.getBucketListHandler)
	router.Methods(http.MethodPost).Path("/bucket/create").HandlerFunc(c.createBucketHandler)
	router.Methods(http.MethodPost).Path("/bucket/delete").HandlerFunc(c.deleteBucketHandler)

	// S3 router for object
	router.Methods(http.MethodPost).Path("/object/put").HandlerFunc(c.putObjectHandler)
	router.Methods(http.MethodPost).Path("/object/get").HandlerFunc(c.getObjectHandler)
	router.Methods(http.MethodPost).Path("/object/delete").HandlerFunc(c.deleteObjectHandler)
	router.Methods(http.MethodPost).Path("/object/list").HandlerFunc(c.getObjectListHandler)
	router.Methods(http.MethodPost).Path("/object/url").HandlerFunc(c.createObjectUrlHandler)

	router.Methods(http.MethodPost).Path("/folder/create").HandlerFunc(c.createFolderHandler)
	router.Methods(http.MethodPost).Path("/folder/list").HandlerFunc(c.listFolderHandler)

	// monitor router

	//  bucket
	//  getBucketList
	//	createBucket
	//	deleteBucket

	//  putObject
	//	getObject
	//	deleteObject
	//	getObjectList
	//	createFolder
	//	listFolder
	//	createUrl

	//  setBucketACL
	//	getBucketACL

}
