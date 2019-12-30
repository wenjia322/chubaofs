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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"encoding/json"
	"github.com/chubaofs/chubaofs/util/log"
	"io/ioutil"
	"net/http"
)

func (c *Console) getBucketListHandler(w http.ResponseWriter, r *http.Request) {
	region := "us-east-1"
	accessKey := "YLWBsakx5hJK4cO4NcwyE72hA9KTGQQ3"
	secretKey := "qpxLnZunpmKlZCyIULprXxoHnVu823Ku"

	s3session, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Endpoint:    aws.String(c.s3Endpoint),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, "23"),
	})
	if err != nil {
		log.LogErrorf("getBucketListHandler : create s3 client session failed cause : %s", err)
		writeErrorResponse(w, "List buckets failed")
		return
	}

	s3Client := s3.New(s3session)
	response, err := s3Client.ListBuckets(nil)
	if err != nil {
		log.LogErrorf("getBucketListHandler : list buckets failed cause : %s", err)
		writeErrorResponse(w, "List buckets failed")
		return
	}

	buckets := make([]*Bucket, 0)
	for _, b := range response.Buckets {
		bucket := &Bucket{Name:*b.Name, CreateTime:*b.CreationDate}
		buckets = append(buckets, bucket)
	}

	writeDataResponse(w, buckets)
}

func (c *Console) createBucketHandler(w http.ResponseWriter, r *http.Request) {
	region := "us-east-1"
	accessKey := "YLWBsakx5hJK4cO4NcwyE72hA9KTGQQ3"
	secretKey := "qpxLnZunpmKlZCyIULprXxoHnVu823Ku"

	var req map[string]interface{}
	body, err := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &req)
	if err != nil {
		log.LogErrorf("createBucketHandler : create bucket failed cause : %s", err)
		writeErrorResponse(w, "Create bucket failed")
		return
	}

	bucketName := req["bucketName"].(string)
	s3session, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Endpoint:    aws.String(c.s3Endpoint),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, "23"),
	})

	s3Client := s3.New(s3session)
	_, err = s3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.LogErrorf("createBucketHandler : create bucket %s failed cause : %s", bucketName, err)
		writeErrorResponse(w, "Create bucket failed")
		return
	}
	writeSuccessResponse(w)
}

func (c *Console) deleteBucketHandler(w http.ResponseWriter, r *http.Request) {
	region := "us-east-1"
	accessKey := "YLWBsakx5hJK4cO4NcwyE72hA9KTGQQ3"
	secretKey := "qpxLnZunpmKlZCyIULprXxoHnVu823Ku"

	var req map[string]interface{}
	body, err := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &req)
	if err != nil {
		log.LogErrorf("deleteBucketHandler : create bucket failed cause : %s", err)
		writeErrorResponse(w, "Create bucket failed")
		return
	}

	bucketName := req["bucketName"].(string)
	s3session, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Endpoint:    aws.String(c.s3Endpoint),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, "23"),
	})

	s3Client := s3.New(s3session)
	_, err = s3Client.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.LogErrorf("deleteBucketHandler : delete bucket %s failed cause : %s", bucketName, err)
		writeErrorResponse(w, "Create bucket failed")
		return
	}

	err = s3Client.WaitUntilBucketNotExists(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})

	writeSuccessResponse(w)
}

func (c *Console) putObjectHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) getObjectHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) deleteObjectHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) getObjectListHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) createObjectUrlHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) createFolderHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) listFolderHandler(w http.ResponseWriter, r *http.Request) {

}
