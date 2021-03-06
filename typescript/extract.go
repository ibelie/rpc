// Copyright 2017 - 2018 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package typescript

import (
	"log"
	"os"
	"path"
	"reflect"

	"encoding/json"
	"os/exec"
)

var JS_PATH = path.Join(os.Getenv("GOPATH"), "src", reflect.TypeOf(PackageStr{}).PkgPath(), "extract.js")

type TypeStr struct {
	Simple  string
	Variant []*TypeStr
	List    *TypeStr
	Key     *TypeStr
	Value   *TypeStr
}

type FieldStr struct {
	Name     string
	Document string
	Type     *TypeStr
}

type MethodStr struct {
	Name     string
	Document string
	Result   *TypeStr
	Params   []*TypeStr
}

type ObjectStr struct {
	Name    string
	Module  string
	Parents []*TypeStr
	Fields  []*FieldStr
	Methods []*MethodStr
}

type PackageStr struct {
	Files   []string
	Objects []*ObjectStr
}

func Extract(files []string) (pkg *PackageStr) {
	output, err := exec.Command("node", append([]string{JS_PATH}, files...)...).CombinedOutput()
	if err != nil {
		log.Fatalf("[Typescript] Cannot extract: %s\n>>>> %v", string(output), err)
	} else if err = json.Unmarshal(output, &pkg); err != nil {
		log.Fatalf("[Typescript] Cannot unmarshal objects:\n>>>> %v", err)
	}
	return
}
