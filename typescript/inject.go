// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package typescript

import (
	"log"
	"regexp"
	"sort"
	"strings"

	"go/build"
	"io/ioutil"

	"github.com/ibelie/tygo"
)

type Entity struct {
	Name       string
	Components []*tygo.Object
}

func Inject(input string, tsOut string, goOut string) {
	ioutil.WriteFile(tsOut+"/rpc.d.ts", []byte(RPC_D_TS), 0666)
	entities := parse(input)
	for _, entity := range entities {
		log.Println(entity.Name)
		for _, component := range entity.Components {
			log.Println("\t", component.Name)
		}
	}
}

func parse(path string) (entities []*Entity) {
	text, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("[RPC][Typescript] Cannot read file:\n>>>>%v", err)
		return nil
	}

	componentReg := regexp.MustCompile(`([_\d\w]+)\s*:\s*([\._\d\w]+)\s*;`)
	entityReg := regexp.MustCompile(`class\s+([_\d\w]+)\s+extends\s+ibelie.rpc.Entity\s*{`)
	entityText := entityReg.Split(string(text), -1)
	entityMap := make(map[string]map[string][]string)
	var entitySorted []string
	for i, e := range entityReg.FindAllStringSubmatch(string(text), -1) {
		entityMap[e[1]] = make(map[string][]string)
		entitySorted = append(entitySorted, e[1])
		for _, c := range componentReg.FindAllStringSubmatch(entityText[i+1], -1) {
			if !strings.HasSuffix(c[2], c[1]) {
				continue
			}
			pkg := strings.Replace(c[2][:len(c[2])-len(c[1])-1], ".", "/", -1)
			entityMap[e[1]][pkg] = append(entityMap[e[1]][pkg], c[1])
		}
	}

	sort.Strings(entitySorted)
	for _, n := range entitySorted {
		var componentSorted []string
		componentMap := make(map[string]*tygo.Object)
		for pkg, names := range entityMap[n] {
			if _, err := build.Import(pkg, "", build.ImportComment); err != nil {
				log.Printf("[RPC][Typescript] Ignore component:\n>>>>%v", err)
				continue
			}
			for _, t := range tygo.Extract(pkg, nil) {
				for _, s := range names {
					if object, ok := t.(*tygo.Object); ok &&
						object.Parent.Name == "Entity" && s == object.Name {
						componentSorted = append(componentSorted, s)
						componentMap[s] = object
					}
				}
			}
		}
		sort.Strings(componentSorted)
		var components []*tygo.Object
		for _, c := range componentSorted {
			components = append(components, componentMap[c])
		}
		entities = append(entities, &Entity{Name: n, Components: components})
	}

	return entities
}

func injectGo(path string, entities []*Entity) {

}

func injectTypescript(path string, entities []*Entity) {

}
