// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"log"

	"github.com/ibelie/rpc/python"
	"github.com/ibelie/tygo"
)

func Python(identName string, input string, pyOut string, ignore []string) (entities []*Entity) {
	pkg := python.Extract(input, pyOut, ignore)
	components := make(map[string]*Component)
	for n, c := range pkg.Components {
		components[n] = &Component{
			Name:    c.Name,
			Path:    c.Package,
			Methods: c.Messages,
		}
	}
	for _, e := range pkg.Entities {
		entity := &Entity{Name: e.Name}
		for _, c := range e.Components {
			if component, ok := components[c]; ok {
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
