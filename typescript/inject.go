// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package typescript

import (
	"io/ioutil"
	"log"
	"regexp"
	"strings"
)

func Inject(input string, tsOut string, goOut string) {
	ioutil.WriteFile(tsOut+"/rpc.d.ts", []byte(RPC_D_TS), 0666)
	entities := parse(input)
	if entities == nil {
		return
	}
	for entity, components := range entities {
		log.Println(entity)
		for pkg, names := range components {
			log.Println("\t", pkg, names)
		}
	}
}

func parse(path string) map[string]map[string][]string {
	text, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("[RPC][Typescript] Cannot read file:\n>>>>%v", err)
		return nil
	}
	entities := make(map[string]map[string][]string)
	componentReg := regexp.MustCompile(`([_\d\w]+): ([\._\d\w]+);`)
	entityReg := regexp.MustCompile(`class\s+([_\d\w]+)\s+extends\s+ibelie.rpc.Entity\s+{`)
	entityText := entityReg.Split(string(text), -1)
	for i, e := range entityReg.FindAllStringSubmatch(string(text), -1) {
		entities[e[1]] = make(map[string][]string)
		for _, c := range componentReg.FindAllStringSubmatch(entityText[i+1], -1) {
			if !strings.HasSuffix(c[2], c[1]) {
				continue
			}
			pkg := strings.Replace(c[2][:len(c[2])-len(c[1])-1], ".", "/", -1)
			entities[e[1]][pkg] = append(entities[e[1]][pkg], c[1])
		}
	}
	return entities
}

func injectGo(path string, entities map[string]map[string][]string) {

}

func injectTypescript(path string, entities map[string]map[string][]string) {

}
