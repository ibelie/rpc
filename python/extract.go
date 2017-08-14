// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package python

import (
	"log"
	"os"
	"path"
	"reflect"

	"encoding/json"
	"os/exec"
)

var PY_PATH = path.Join(os.Getenv("GOPATH"), "src", reflect.TypeOf(PackageStr{}).PkgPath(), "extract.py")

type TypeStr struct {
	Simple string
	List   *TypeStr
	Key    *TypeStr
	Value  *TypeStr
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
	Parents []*TypeStr
	Fields  []*FieldStr
	Methods []*MethodStr
}

type PackageStr struct {
	Files   []string
	Objects []*ObjectStr
}

func Extract(file string) (pkg *PackageStr) {
	output, err := exec.Command("python", PY_PATH, file).CombinedOutput()
	log.Println(string(output))
	if err != nil {
		log.Fatalf("[Python] Cannot extract: %s\n>>>> %v", string(output))
	} else if err = json.Unmarshal(output, &pkg); err != nil {
		log.Fatalf("[Python] Cannot unmarshal objects:\n>>>> %v", err)
	}
	return
}
