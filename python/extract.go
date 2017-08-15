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

type ComponentStr struct {
	Name       string
	Package    string
	Properties []string
	Messages   []string
}

type EntityStr struct {
	Name       string
	Components []string
}

type PackageStr struct {
	Entities   []*EntityStr
	Components map[string]*ComponentStr
}

func Extract(input string, pyOut string, ignore []string) (pkg *PackageStr) {
	args := append([]string{PY_PATH, input, path.Join(pyOut, "microserver.proto")}, ignore...)
	output, err := exec.Command("python", args...).CombinedOutput()
	if err != nil {
		log.Fatalf("[Python] Cannot extract: %s\n>>>> %v", string(output))
	} else if err = json.Unmarshal(output, &pkg); err != nil {
		log.Fatalf("[Python] Cannot unmarshal objects:\n>>>> %v", err)
	}
	return
}
