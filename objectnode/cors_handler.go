package objectnode

// https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/dev/EnableCorsUsingREST.html

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/chubaofs/chubaofs/util/log"
)

// https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetBucketCors.html
func (o *ObjectNode) getBucketCorsHandler(w http.ResponseWriter, r *http.Request) {
	log.LogInfof("Get bucket cors")

	var err error
	var param = ParseRequestParam(r)
	if param.Bucket() == "" {
		_ = NoSuchBucket.ServeResponse(w, r)
		return
	}

	var vol *Volume
	if vol, err = o.vm.Volume(param.Bucket()); err != nil {
		_ = NoSuchBucket.ServeResponse(w, r)
		return
	}

	corsOutput := &GetBucketCorsOutput{}
	cors := vol.loadCors()
	if cors != nil {
		corsOutput.corsRules = cors.CORSRule
	}
	var corsData []byte
	if corsData, err = xml.Marshal(corsOutput); err != nil {
		_ = InternalErrorCode(err).ServeResponse(w, r)
		return
	}

	if _, err = w.Write(corsData); err != nil {
		log.LogErrorf("getBucketCorsHandler: write response body fail, requestID(%v) err(%v)", GetRequestID(r), err)
		_ = InternalErrorCode(err).ServeResponse(w, r)
	}

	return
}

// https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutBucketCors.html
func (o *ObjectNode) putBucketCorsHandler(w http.ResponseWriter, r *http.Request) {
	log.LogInfof("Put bucket cors")

	var err error
	var param = ParseRequestParam(r)
	if param.Bucket() == "" {
		_ = NoSuchBucket.ServeResponse(w, r)
		return
	}
	var vol *Volume
	if vol, err = o.vm.Volume(param.Bucket()); err != nil {
		_ = NoSuchBucket.ServeResponse(w, r)
		return
	}

	var bytes []byte
	if bytes, err = ioutil.ReadAll(r.Body); err != nil && err != io.EOF {
		_ = InternalErrorCode(err).ServeResponse(w, r)
		return
	}

	var corsConfig *CORSConfiguration
	if corsConfig, err = parseCorsConfig(bytes); err != nil {
		_ = InvalidArgument.ServeResponse(w, r)
		return
	}
	if corsConfig == nil {
		_ = InvalidArgument.ServeResponse(w, r)
		return
	}

	var newBytes []byte
	if newBytes, err = json.Marshal(corsConfig); err != nil {
		_ = InternalErrorCode(err).ServeResponse(w, r)
		return
	}
	if err = storeBucketCors(newBytes, vol); err != nil {
		_ = InternalErrorCode(err).ServeResponse(w, r)
		return
	}
	vol.storeCors(corsConfig)

	return
}

// https://docs.aws.amazon.com/AmazonS3/latest/API/API_DeleteBucketCors.html
func (o *ObjectNode) deleteBucketCorsHandler(w http.ResponseWriter, r *http.Request) {
	log.LogInfof("Delete bucket cors")

	var err error
	var param = ParseRequestParam(r)
	if param.Bucket() == "" {
		_ = NoSuchBucket.ServeResponse(w, r)
		return
	}
	var vol *Volume
	if vol, err = o.vm.Volume(param.Bucket()); err != nil {
		_ = NoSuchBucket.ServeResponse(w, r)
		return
	}

	if err = deleteBucketCors(vol); err != nil {
		_ = InternalErrorCode(err).ServeResponse(w, r)
		return
	}
	vol.storeCors(nil)

	w.WriteHeader(http.StatusNoContent)

	return
}

//https://docs.aws.amazon.com/AmazonS3/latest/API/RESTOPTIONSobject.html
func (o *ObjectNode) optionsObjectHandler(w http.ResponseWriter, r *http.Request) {
	log.LogInfof("optionsObjectHandler: OPTIONS object, requestID(%v) remote(%v)", GetRequestID(r), r.RemoteAddr)
	// Already done in methods 'corsMiddleware'.
	return
}
