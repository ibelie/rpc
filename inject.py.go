// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"log"
	"strings"

	"github.com/ibelie/rpc/python"
	"github.com/ibelie/tygo"
)

func Python(identName string, input string, tsOut string) (entities []*Entity) {
	pkg := python.Extract(input)
	components := make(map[string]*Component)
	for _, o := range pkg.Objects {
		if len(o.Parents) != 1 || o.Parents[0] == nil || o.Parents[0].Simple != "ibelie.rpc.Component" {
			continue
		}
		component := &Component{Name: o.Name}
		for _, m := range o.Methods {
			component.Methods = append(component.Methods, m.Name)
		}
		components[component.Name] = component
	}
	for _, o := range pkg.Objects {
		if len(o.Parents) != 1 || o.Parents[0] == nil || o.Parents[0].Simple != "ibelie.rpc.Entity" {
			continue
		}
		entity := &Entity{Name: o.Name}
		for _, f := range o.Fields {
			if f.Type == nil || !strings.HasSuffix(f.Type.Simple, "."+f.Name) {
				continue
			} else if component, ok := components[f.Name]; ok {
				component.Path = strings.Replace(f.Type.Simple[:len(f.Type.Simple)-len(f.Name)-1], ".", "/", -1)
				entity.Components = append(entity.Components, component)
			}
		}
		entities = append(entities, entity)
	}

	types := resolveEntities(entities)
	objects := make(map[string]*tygo.Object)
	for _, t := range types {
		if object, ok := t.(*tygo.Object); ok {
			if o, exist := objects[object.Name]; exist {
				log.Fatalf("[RPC][Python] Object already exists: %v %v", o, object)
			}
			objects[object.Name] = object
		}
	}

	return entities
}
