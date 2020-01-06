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
	"context"
	"github.com/chubaofs/chubaofs/sdk/master"
	"github.com/chubaofs/chubaofs/util/config"
	"github.com/chubaofs/chubaofs/util/errors"
	"github.com/chubaofs/chubaofs/util/log"
	"github.com/gorilla/mux"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"
	"github.com/chubaofs/chubaofs/util/auth"
	"time"
	authSdk "github.com/chubaofs/chubaofs/sdk/auth"
	"strings"
)

// The status of the console server
const (
	Standby  uint32 = iota
	Start
	Running
	Shutdown
	Stopped
)

// Configuration keys
const (
	configListen      = "listen"
	configMasterNodes = "masterAddr"
	configS3Endpoint  = "s3Endpoint"
	configConsoleId   = "consoleId"
	configConsoleKey  = "consoleKey"
	configAuthNodes   = "authAddr"
)

// Default of configuration value
const (
	defaultListen = ":80"
)

var (
	regexpListen = regexp.MustCompile("^(([0-9]{1,3}.){3}([0-9]{1,3}))?:(\\d)+$")
)

type Console struct {
	listen       string
	s3Region     string
	s3Endpoint   string
	httpServer   *http.Server
	state        uint32
	wg           sync.WaitGroup
	masterClient *master.MasterClient
	authClient   *authSdk.AuthClient
	authNodeInfo *AuthNodeInfo
	consoleId    string
	consoleKey   string
}

type AuthNodeInfo struct {
	authAddr  []string
	authId    string
	ticket    *auth.Ticket
	ticketTTL int
	rwMutex   sync.RWMutex
}

func (c *Console) Start(cfg *config.Config) (err error) {
	if atomic.CompareAndSwapUint32(&c.state, Standby, Start) {
		defer func() {
			if err != nil {
				atomic.StoreUint32(&c.state, Standby)
			} else {
				atomic.StoreUint32(&c.state, Running)
			}
		}()
		if err = c.startHandle(cfg); err != nil {
			return
		}
		c.wg.Add(1)
	}
	return
}

func (c *Console) startHandle(cfg *config.Config) (err error) {
	// parse config
	if err = c.parseConfig(cfg); err != nil {
		return
	}
	// start rest api
	if err = c.startMuxRestAPI(); err != nil {
		log.LogInfof("handleStart: start mux rest api fail, err(%v)", err)
		return
	}
	log.LogInfo("console node start success")
	return
}

func (c *Console) startMuxRestAPI() (err error) {
	router := mux.NewRouter().SkipClean(true)
	c.registerApiRouters(router)
	router.Use(
		c.authMiddleware,
	)

	var server = &http.Server{
		Addr:    c.listen,
		Handler: router,
	}

	go func() {
		if err = server.ListenAndServe(); err != nil {
			log.LogErrorf("startMuxRestAPI: start http server fail, err(%s)", err)
			return
		}
	}()
	c.httpServer = server
	return
}

func (c *Console) Shutdown() {
	if atomic.CompareAndSwapUint32(&c.state, Running, Shutdown) {
		c.shutdownRestAPI()
		c.wg.Done()
		atomic.StoreUint32(&c.state, Stopped)
	}
}

func (c *Console) shutdownRestAPI() {
	if c.httpServer != nil {
		_ = c.httpServer.Shutdown(context.Background())
		c.httpServer = nil
	}
}

func (c *Console) Sync() {
	if atomic.LoadUint32(&c.state) == Running {
		c.wg.Wait()
	}
}

func (c *Console) parseConfig(cfg *config.Config) (err error) {
	// parse listen
	listen := cfg.GetString(configListen)
	if len(listen) == 0 {
		listen = defaultListen
	}

	// parse s3 endpoint
	endpoint := cfg.GetString(configS3Endpoint)
	if len(endpoint) == 0 {
		log.LogErrorf("parseConfig: s3 endpoint is empty")
	}
	if match := regexpListen.MatchString(listen); !match {
		return errors.New("invalid listen configuration")
	}

	// parse master nodes
	masterNodes := make([]string, 0)
	if len(cfg.GetArray(configMasterNodes)) == 0 {
		return errors.New("Err:masterAddr invalid")
	}
	for _, ip := range cfg.GetArray(configMasterNodes) {
		masterNodes = append(masterNodes, ip.(string))
	}
	masterClient := master.NewMasterClient(masterNodes, false)

	// get s3 region
	ci, err := masterClient.AdminAPI().GetClusterInfo()
	if err != nil || len(ci.Cluster) <= 0 {
		return errors.New("Err:cluster info invalid")
	}

	// parse auth node info
	consoleId := cfg.GetString(configConsoleId)
	consoleKey := cfg.GetString(configConsoleKey)
	if len(consoleId) == 0 {
		log.LogErrorf("parseConfig: console id is empty")
		return errors.New("console id is empty")
	}
	if len(consoleKey) == 0 {
		log.LogErrorf("parseConfig: console key is empty")
		return errors.New("console key is empty")
	}
	authNodes := make([]string, 0)
	if len(cfg.GetArray(configAuthNodes)) == 0 {
		return errors.New("Err:authAddr invalid")
	}
	for _, ip := range cfg.GetArray(configAuthNodes) {
		authNodes = append(authNodes, ip.(string))
	}
	authClient := authSdk.NewAuthClient(strings.Join(authNodes, ","), false,  "")

	c.listen = listen
	c.s3Region = ci.Cluster
	c.s3Endpoint = endpoint
	c.authClient = authClient
	c.masterClient = masterClient
	return
}

func (c *Console) updateAuthTicket() {
	ttl := c.authNodeInfo.ticketTTL
	ttl = ttl - 10

	duration := time.Duration(time.Minute * time.Duration(ttl))
	ticker := time.NewTicker(duration)

	doUpdate := func() (*auth.Ticket, error) {
		c.authNodeInfo.rwMutex.Lock()
		defer c.authNodeInfo.rwMutex.Unlock()
		id := c.consoleId
		userKey := c.consoleKey
		serviceId := c.authNodeInfo.authId

		return c.authClient.API().GetTicket(id, userKey, serviceId)
	}

	for {
		select {
		case timeNow := <-ticker.C:
			log.LogInfof("Update auth ticket, current time : %s", timeNow)
			ticket, err := doUpdate()
			if err != nil {
				log.LogErrorf("Get auth node ticket failed cause : %s", err)
			}
			c.authNodeInfo.ticket = ticket
		}
	}
}

func NewServer() *Console {
	return &Console{}
}
