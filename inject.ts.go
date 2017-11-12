// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"io/ioutil"

	"github.com/ibelie/rpc/typescript"
	"github.com/ibelie/tygo"
)

func Typescript(identName string, tsOut string, inputs []string) (entities []*Entity) {
	pkg := typescript.Extract(inputs)
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
	tygo.Typescript(tsOut, "types", "ibelie.rpc", types, PROP_PRE)
	injectJavascript(identName, tsOut, entities)
	injectTypescript(tsOut, entities, types)
	return entities
}

func injectTypescript(dir string, entities []*Entity, types []tygo.Type) {
	var buffer bytes.Buffer
	var methods []string
	methodsMap := make(map[string]bool)
	tygo.TS_OBJECTS = tygo.ObjectMap(types)
	for _, e := range entities {
		for _, c := range e.Components {
			if c.Protocol == nil {
				continue
			}
			for _, m := range c.Protocol.Methods {
				if ok, exist := methodsMap[m.Name]; exist && ok {
					continue
				} else if len(m.Results) > 0 {
					continue
				}
				var params []string
				for i, p := range m.Params {
					params = append(params, fmt.Sprintf("a%d: %s", i, p.Typescript()))
				}
				methods = append(methods, fmt.Sprintf(`
		%s(%s): void;`, m.Name, strings.Join(params, ", ")))
				methodsMap[m.Name] = true
			}
		}
	}
	tygo.TS_OBJECTS = nil

	buffer.Write([]byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

declare module ibelie.rpc {
	interface Entity {
		__class__: string;
		isAwake: boolean;
		ByteSize(): number;
		Serialize(): Uint8Array;
		Deserialize(data: Uint8Array): void;%s
	}

	class Component {
		Entity: Entity;
		Awake(entity: Entity): any;
		Drop(entity: Entity): any;
	}

	class Connection {
		socket: WebSocket;
		entities: {[index: string]: Entity};
		constructor(url: string);
		disconnect(): void;
	}
}
`, strings.Join(methods, ""))))

	ioutil.WriteFile(path.Join(dir, "rpc.d.ts"), buffer.Bytes(), 0666)
}
