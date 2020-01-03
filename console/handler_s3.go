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
	"fmt"
	"github.com/chubaofs/chubaofs/util/log"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
)

func (c *Console) getS3Keys(r *http.Request) (string, string, error) {
	// parse query parameter
	params := r.URL.Query()
	userId, _ := params["userId"]
	fmt.Println("userId : ", userId)
	// get access key and secret key using user id from auth node
	return "", "", nil
}

func (c *Console) getS3Client(w http.ResponseWriter, r *http.Request) (*s3.S3, error) {
	accessKey, secretKey, err := c.getS3Keys(r)
	if err != nil {
		log.LogErrorf("getS3Client : get user info failed cause : %s", err)
		return nil, err
	}

	s3session, err := session.NewSession(&aws.Config{
		Region:      aws.String(c.s3Region),
		Endpoint:    aws.String(c.s3Endpoint),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, "23"),
	})
	if err != nil {
		log.LogErrorf("getS3Client : create s3 client session failed cause : %s", err)
		return nil, err
	}

	return s3.New(s3session), nil
}

func (c *Console) getBucketListHandler(w http.ResponseWriter, r *http.Request) {
	// Get s3 client
	s3Client, err := c.getS3Client(w, r)
	if err != nil {
		log.LogErrorf("getBucketListHandler : Get s3 client failed cause : %s", err)
		writeErrorResponse(w, "Get s3 client failed")
		return
	}

	response, err := s3Client.ListBuckets(nil)
	if err != nil {
		log.LogErrorf("getBucketListHandler : list buckets failed cause : %s", err)
		writeErrorResponse(w, "List buckets failed")
		return
	}

	buckets := make([]*Bucket, 0)
	for _, b := range response.Buckets {
		bucket := &Bucket{Name: *b.Name, CreateTime: *b.CreationDate}
		buckets = append(buckets, bucket)
	}

	writeDataResponse(w, buckets)
}

func (c *Console) createBucketHandler(w http.ResponseWriter, r *http.Request) {
	// get bucket name request parameter
	var req map[string]interface{}
	body, err := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &req)
	if err != nil {
		log.LogErrorf("createBucketHandler : get bucket name failed cause : %s", err)
		writeErrorResponse(w, "Create bucket failed")
		return
	}
	bucketName := req["bucketName"].(string)

	// Get s3 client
	s3Client, err := c.getS3Client(w, r)
	if err != nil {
		log.LogErrorf("createBucketHandler : get s3 client failed while create bucket %s cause : %s", bucketName, err)
		writeErrorResponse(w, "Get s3 client failed")
		return
	}

	_, err = s3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.LogErrorf("createBucketHandler : create bucket %s failed cause : %s", bucketName, err)
		writeErrorResponse(w, "Create bucket failed")
		return
	}

	log.LogInfof("Create bucket %s success", bucketName)
	writeSuccessResponse(w)
}

func (c *Console) deleteBucketHandler(w http.ResponseWriter, r *http.Request) {
	// get bucket name from request parameter
	var req map[string]interface{}
	body, err := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &req)
	if err != nil {
		log.LogErrorf("deleteBucketHandler : get bucket name failed cause : %s", err)
		writeErrorResponse(w, "Get bucket name failed")
		return
	}
	bucketName := req["bucketName"].(string)

	// Get s3 client
	s3Client, err := c.getS3Client(w, r)
	if err != nil {
		log.LogErrorf("deleteBucketHandler : get s3 client failed while deleting bucket %s cause : %s", bucketName, err)
		writeErrorResponse(w, "Get s3 client failed")
		return
	}

	_, err = s3Client.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.LogErrorf("deleteBucketHandler : delete bucket %s failed cause : %s", bucketName, err)
		writeErrorResponse(w, "Delete bucket failed")
		return
	}

	err = s3Client.WaitUntilBucketNotExists(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})

	log.LogInfof("Delete bucket %s success", bucketName)
	writeSuccessResponse(w)
}

func (c *Console) putObjectHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(102400)
	fmt.Println(r.MultipartForm.Value)

	bucketName := r.MultipartForm.Value["bucketName"][0]
	objectName := r.MultipartForm.Value["objectName"][0]

	file, _, err := r.FormFile("file")
	if err != nil {
		log.LogErrorf("putObjectHandler : get file from request failed cause : %s", err)
		writeErrorResponse(w, "Put object failed")
		return
	}
	defer file.Close()

	// Get s3 client
	s3Client, err := c.getS3Client(w, r)
	if err != nil {
		log.LogErrorf("putObjectHandler : Get s3 client failed while putting object %s cause : %s", objectName, err)
		writeErrorResponse(w, "Get s3 client failed")
		return
	}

	output, err := s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
		Body:   file,
	})

	if err != nil {
		log.LogErrorf("putObjectHandler : put object %s to bucket %s failed cause : %s", objectName, bucketName, err)
		writeErrorResponse(w, "Put object failed")
		return
	}
	log.LogInfof("Put object %s success, and ETag : %s", objectName, aws.StringValue(output.ETag))
	writeSuccessResponse(w)
}

func (c *Console) getObjectHandler(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	body, err := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &req)
	if err != nil {
		log.LogErrorf("getObjectHandler : Unmarshal request body failed cause : %s", err)
		writeErrorResponse(w, "Get object parameters failed")
		return
	}
	bucketName := req["bucketName"].(string)
	objectName := req["objectName"].(string)

	// Get s3 client
	s3Client, err := c.getS3Client(w, r)
	if err != nil {
		log.LogErrorf("getObjectHandler : Get s3 client failed while getting object cause : %s", err)
		writeErrorResponse(w, "Get s3 client failed")
		return
	}

	// check object is whether existed
	headOutput, err := s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	if err != nil {
		log.LogErrorf("getObjectHandler : check object %s is whether existed failed cause : %s", objectName, err)
		return
	}
	size := headOutput.ContentLength

	getObjectOutput, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	responseData := getObjectOutput.Body
	defer responseData.Close()

	if err != nil {
		log.LogErrorf("getObjectHandler : get object %s from bucket %s failed cause : %s", objectName, bucketName, err)
		writeErrorResponse(w, "Get object failed")
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
	w.Header().Set("Content-Disposition", "attachment; filename="+objectName)
	w.Header().Set("Content-Length", strconv.FormatInt(*size, 10))

	io.Copy(w, responseData)
}

func (c *Console) deleteObjectHandler(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	body, err := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &req)
	if err != nil {
		log.LogErrorf("deleteObjectHandler : Unmarshal request body failed cause : %s", err)
		writeErrorResponse(w, "Get object parameters from request failed")
		return
	}
	bucketName := req["bucketName"].(string)
	objectName := req["objectName"].(string)

	// Get s3 client
	s3Client, err := c.getS3Client(w, r)
	if err != nil {
		log.LogErrorf("deleteObjectHandler : Get s3 client failed while deleting object cause : %s", err)
		writeErrorResponse(w, "Get s3 client failed")
		return
	}

	_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	if err != nil {
		log.LogErrorf("deleteObjectHandler : delete object %s from bucket %s failed cause : %s", objectName, bucketName, err)
		writeErrorResponse(w, "Delete object failed")
		return
	}

	log.LogInfof("Delete object %s success", objectName)
	writeSuccessResponse(w)
}

func (c *Console) getObjectListHandler(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	body, err := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &req)
	if err != nil {
		log.LogErrorf("getObjectListHandler : Unmarshal request body failed cause : %s", err)
		writeErrorResponse(w, "Get bucket name from request failed")
		return
	}
	prefix := req["prefix"].(string)
	maxKeys := req["maxKeys"].(string)
	startAfter := req["startAfter"].(string)
	bucketName := req["bucketName"].(string)

	// Get s3 client
	s3Client, err := c.getS3Client(w, r)
	if err != nil {
		log.LogErrorf("getObjectListHandler : Get s3 client failed while deleting object cause : %s", err)
		writeErrorResponse(w, "Get s3 client failed")
		return
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	}
	if len(prefix) > 0 {
		input.SetPrefix(prefix)
	}
	if len(startAfter) > 0 {
		input.SetStartAfter(startAfter)
	}
	maxKeysInt, _ := strconv.ParseInt(S3ListObjectsMaxKeys, 10, 64)
	if len(maxKeys) > 0 {
		maxKeysInt, err = strconv.ParseInt(maxKeys, 10, 64)
		if err != nil {
			log.LogErrorf("Parse max keys from request failed cause : %s", err)
		}
	}
	input.SetMaxKeys(maxKeysInt)

	output, err := s3Client.ListObjectsV2(input)
	if err != nil {
		log.LogErrorf("getObjectListHandler : get object list from bucket %s failed cause : %s", bucketName, err)
		writeErrorResponse(w, "Delete object failed")
		return
	}

	objects := make([]*Object, 0)
	for _, o := range output.Contents {
		object := &Object{
			Size:         aws.Int64Value(o.Size),
			OwnerId:      aws.StringValue(o.Owner.ID),
			OwnerName:    aws.StringValue(o.Owner.DisplayName),
			ObjectName:   aws.StringValue(o.Key),
			StorageClass: aws.StringValue(o.StorageClass),
			LastModified: o.LastModified,
		}
		objects = append(objects, object)
	}

	directories := make([]*string, 0)
	for _, p := range output.CommonPrefixes {
		directories = append(directories, p.Prefix)
	}

	objectList := ObjectList{
		KeyCount:    aws.Int64Value(output.KeyCount),
		StartAfter:  aws.StringValue(output.StartAfter),
		IsTruncated: aws.BoolValue(output.IsTruncated),
		Objects:     objects,
		Directories: directories,
	}
	writeDataResponse(w, objectList)
}

func (c *Console) createObjectUrlHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) createFolderHandler(w http.ResponseWriter, r *http.Request) {

}

func (c *Console) listFolderHandler(w http.ResponseWriter, r *http.Request) {

}
