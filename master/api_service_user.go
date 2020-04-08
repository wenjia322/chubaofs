package master

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/chubaofs/chubaofs/proto"
	"github.com/chubaofs/chubaofs/util/errors"
	"github.com/chubaofs/chubaofs/util/log"
)

func (m *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var (
		userInfo *proto.UserInfo
		err      error
	)
	var bytes []byte
	if bytes, err = ioutil.ReadAll(r.Body); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	var param = proto.UserCreateParam{}
	if err = json.Unmarshal(bytes, &param); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	if !ownerRegexp.MatchString(param.ID) {
		sendErrReply(w, r, newErrHTTPReply(proto.ErrInvalidUserID))
		return
	}
	if param.Type == proto.UserTypeRoot {
		sendErrReply(w, r, newErrHTTPReply(proto.ErrInvalidUserType))
		return
	}
	if userInfo, err = m.user.createKey(&param); err != nil {
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	_ = sendOkReply(w, r, newSuccessHTTPReply(userInfo))
}

func (m *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	var (
		userID string
		err    error
	)
	if userID, err = parseUser(r); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	if err = m.user.deleteKey(userID); err != nil {
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	msg := fmt.Sprintf("delete user[%v] successfully", userID)
	log.LogWarn(msg)
	sendOkReply(w, r, newSuccessHTTPReply(msg))
}

func (m *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	var (
		userInfo *proto.UserInfo
		err      error
	)
	var bytes []byte
	if bytes, err = ioutil.ReadAll(r.Body); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	var param = proto.UserUpdateParam{}
	if err = json.Unmarshal(bytes, &param); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	if param.Type == proto.UserTypeRoot {
		sendErrReply(w, r, newErrHTTPReply(proto.ErrInvalidUserType))
		return
	}
	if userInfo, err = m.user.updateKey(&param); err != nil {
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	_ = sendOkReply(w, r, newSuccessHTTPReply(userInfo))
}

func (m *Server) getUserAKInfo(w http.ResponseWriter, r *http.Request) {
	var (
		ak       string
		userInfo *proto.UserInfo
		err      error
	)
	if ak, err = parseAccessKey(r); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	if userInfo, err = m.user.getKeyInfo(ak); err != nil {
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	sendOkReply(w, r, newSuccessHTTPReply(userInfo))
}

func (m *Server) getUserInfo(w http.ResponseWriter, r *http.Request) {
	var (
		userID   string
		userInfo *proto.UserInfo
		err      error
	)
	if userID, err = parseUser(r); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	if userInfo, err = m.user.getUserInfo(userID); err != nil {
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	sendOkReply(w, r, newSuccessHTTPReply(userInfo))
}

func (m *Server) updateUserPolicy(w http.ResponseWriter, r *http.Request) {
	var (
		userInfo *proto.UserInfo
		bytes    []byte
		err      error
	)
	if bytes, err = ioutil.ReadAll(r.Body); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	var param = proto.UserPermUpdateParam{}
	if err = json.Unmarshal(bytes, &param); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	if _, err = m.cluster.getVol(param.Volume); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeVolNotExists, Msg: err.Error()})
		return
	}
	if userInfo, err = m.user.updatePolicy(&param); err != nil {
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	sendOkReply(w, r, newSuccessHTTPReply(userInfo))
}

func (m *Server) removeUserPolicy(w http.ResponseWriter, r *http.Request) {
	var (
		userInfo *proto.UserInfo
		bytes    []byte
		err      error
	)
	if bytes, err = ioutil.ReadAll(r.Body); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	var param = proto.UserPermRemoveParam{}
	if err = json.Unmarshal(bytes, &param); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	if _, err = m.cluster.getVol(param.Volume); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeVolNotExists, Msg: err.Error()})
		return
	}
	if userInfo, err = m.user.removePolicy(&param); err != nil {
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	sendOkReply(w, r, newSuccessHTTPReply(userInfo))
}

func (m *Server) deleteUserVolPolicy(w http.ResponseWriter, r *http.Request) {
	var (
		vol string
		err error
	)
	if vol, err = parseVolName(r); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	if err = m.user.deleteVolPolicy(vol); err != nil {
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	msg := fmt.Sprintf("delete vol[%v] policy successfully", vol)
	log.LogWarn(msg)
	sendOkReply(w, r, newSuccessHTTPReply(msg))
}

func (m *Server) transferUserVol(w http.ResponseWriter, r *http.Request) {
	var (
		bytes    []byte
		vol      *Vol
		userInfo *proto.UserInfo
		err      error
	)
	if bytes, err = ioutil.ReadAll(r.Body); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	var param = proto.UserTransferVolParam{}
	if err = json.Unmarshal(bytes, &param); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	if vol, err = m.cluster.getVol(param.Volume); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeVolNotExists, Msg: err.Error()})
		return
	}
	if !param.Force && vol.Owner != param.UserSrc {
		sendErrReply(w, r, newErrHTTPReply(proto.ErrHaveNoPolicy))
		return
	}
	if userInfo, err = m.user.transferVol(&param); err != nil {
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	owner := vol.Owner
	vol.Owner = userInfo.UserID
	if err = m.cluster.syncUpdateVol(vol); err != nil {
		vol.Owner = owner
		err = proto.ErrPersistenceByRaft
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	sendOkReply(w, r, newSuccessHTTPReply(userInfo))
}

func (m *Server) getAllUsers(w http.ResponseWriter, r *http.Request) {
	var (
		keywords string
		users    []*proto.UserInfo
		err      error
	)
	if keywords, err = parseKeywords(r); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	users = m.user.getAllUserInfo(keywords)
	sendOkReply(w, r, newSuccessHTTPReply(users))
}

func (m *Server) getUserPassword(w http.ResponseWriter, r *http.Request) {
	var (
		ak       string
		password string
		err      error
	)
	if ak, err = parseAccessKey(r); err != nil {
		sendErrReply(w, r, &proto.HTTPReply{Code: proto.ErrCodeParamError, Msg: err.Error()})
		return
	}
	if password, err = m.user.getPassword(ak); err != nil {
		sendErrReply(w, r, newErrHTTPReply(err))
		return
	}
	sendOkReply(w, r, newSuccessHTTPReply(password))
}

func parseUser(r *http.Request) (userID string, err error) {
	if err = r.ParseForm(); err != nil {
		return
	}
	if userID, err = extractUser(r); err != nil {
		return
	}
	return
}

func extractUser(r *http.Request) (user string, err error) {
	if user = r.FormValue(userKey); user == "" {
		err = keyNotFound(userKey)
		return
	}
	return
}

func parseAccessKey(r *http.Request) (ak string, err error) {
	if err = r.ParseForm(); err != nil {
		return
	}
	if ak, err = extractAccessKey(r); err != nil {
		return
	}
	return
}

func parseKeywords(r *http.Request) (keywords string, err error) {
	if err = r.ParseForm(); err != nil {
		return
	}
	keywords = extractKeywords(r)
	return
}

func extractAccessKey(r *http.Request) (ak string, err error) {
	if ak = r.FormValue(akKey); ak == "" {
		err = keyNotFound(akKey)
		return
	}
	if !proto.AKRegexp.MatchString(ak) {
		return "", errors.New("accesskey can only be number and letters")
	}
	return
}

func extractKeywords(r *http.Request) (keywords string) {
	keywords = r.FormValue(keywordsKey)
	return
}
