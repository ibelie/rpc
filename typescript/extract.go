// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package typescript

import (
	"log"
	"os"
	"os/exec"
	"path"
	"reflect"

	"github.com/ibelie/rpc"
)

type X struct{}

var JS_PATH = path.Join(os.Getenv("GOPATH"), "src", reflect.TypeOf(X{}).PkgPath(), "extract.js")

func Extract(file string) (entities []*rpc.Entity) {
	log.Println(JS_PATH)
	cmd := exec.Command("node", JS_PATH, file)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Panicf("[Protobuf] Cannot build proto:\n%v", string(output))
	} else {
		log.Println(string(output))
	}
	return
}
