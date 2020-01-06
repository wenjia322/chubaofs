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
	"fmt"
	"github.com/chubaofs/chubaofs/util/config"
	"github.com/chubaofs/chubaofs/util/log"
	"os"
	"testing"
	"time"
)

func TestConsole_Lifecycle(t *testing.T) {
	var err error
	cfgStr := `
{
	"listen": ":10000",
	"logDir": "/tmp/Logs/chubaofs",
	"masterAddr": [
    	"172.20.240.95:7002",
    	"172.20.240.94:7002"
	],
    "s3Endpoint": "http://s3.jvs.jd.com"
}
`
	// test log
	cfg := config.LoadConfigString(cfgStr)
	if _, err := log.InitLog(cfg.GetString("logDir"), "console", log.DebugLevel, nil); err != nil {
		fmt.Println("Fatal: failed to init log - ", err)
		os.Exit(1)
		return
	}

	console := NewServer()
	if err = console.Start(cfg); err != nil {
		t.Fatalf("start console server fail cause: %v", err)
	}
	go func() {
		fmt.Printf("console server will be shutdown after 300 seconds.\n")
		time.Sleep(300 * time.Second)
		fmt.Printf("console server will be shutdown.\n")
		console.Shutdown()
	}()
	console.Sync()
}
