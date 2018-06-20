// Copyright 2017 - 2018 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"io/ioutil"

	"github.com/ibelie/rpc/python"
	"github.com/ibelie/tygo"
)

const PY_HEADER = `#-*- coding: utf-8 -*-
# Generated by ibelie-rpc.  DO NOT EDIT!
`

func Python(identName string, input string, pyOut string, goOut string, ignore []string) {
	var entities []*Entity
	pkg := python.Extract(input, pyOut, ignore)
	components := make(map[string]*Component)
	for n, c := range pkg.Components {
		components[n] = &Component{
			Name: c.Name,
			Path: c.Package,
			// Methods: c.Messages,
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
	var methods []tygo.Type
	for _, t := range types {
		if object, ok := t.(*tygo.Object); ok {
			for _, field := range object.VisibleFields() {
				methods = append(methods, &tygo.Object{
					Name:   fmt.Sprintf("%s_%s", object.Name, field.Name),
					Parent: &tygo.InstanceType{PkgName: "tygo", PkgPath: tygo.TYGO_PATH, Name: "Tygo"},
					Fields: []*tygo.Field{field},
				})
			}

			for _, method := range object.Methods {
				if len(method.Params) > 0 {
					var params []*tygo.Field
					for i, p := range method.Params {
						params = append(params, &tygo.Field{Type: p, Name: fmt.Sprintf("a%d", i)})
					}
					methods = append(methods, &tygo.Object{
						Name:   fmt.Sprintf("%s_%sParam", object.Name, method.Name),
						Parent: &tygo.InstanceType{PkgName: "tygo", PkgPath: tygo.TYGO_PATH, Name: "Tygo"},
						Fields: params,
					})
				}
				if len(method.Results) > 0 {
					var results []*tygo.Field
					for i, r := range method.Results {
						results = append(results, &tygo.Field{Type: r, Name: fmt.Sprintf("a%d", i)})
					}
					methods = append(methods, &tygo.Object{
						Name:   fmt.Sprintf("%s_%sResult", object.Name, method.Name),
						Parent: &tygo.InstanceType{PkgName: "tygo", PkgPath: tygo.TYGO_PATH, Name: "Tygo"},
						Fields: results,
					})
				}
			}
		}
	}
	types = append(types, methods...)

	symbol_s, symbol_b := entityProxy(identName, goOut, entities)
	var versionBytes []string
	for _, v := range symbol_b {
		versionBytes = append(versionBytes, fmt.Sprintf("\\x%02x", v))
	}

	var buffer bytes.Buffer
	buffer.Write([]byte(PY_HEADER))
	buffer.Write([]byte(fmt.Sprintf(`
IDType = '%s'

Version = '%s'

Symbols = [
	'%s',
]

from microserver.classes import Entity
`, identName, strings.Join(versionBytes, ""), strings.Join(symbol_s, "',\n\t'"))))
	buffer.Write(tygo.Python(types))
	ioutil.WriteFile(path.Join(pyOut, "proto.py"), buffer.Bytes(), 0666)

	buffer.Truncate(0)
	buffer.Write([]byte(PY_HEADER))
	buffer.Write([]byte(fmt.Sprintf(`
IDType = '%s'
`, identName)))
	buffer.Write(tygo.Typyd(types))
	ioutil.WriteFile(path.Join(pyOut, "_proto.py"), buffer.Bytes(), 0666)
}
