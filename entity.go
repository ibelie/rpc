// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"sort"

	"io/ioutil"

	"github.com/ibelie/tygo"
)

var (
	ENTITY_PKG = map[string]string{
		"github.com/ibelie/ruid": "",
		"github.com/ibelie/tygo": "",
	}
)

const (
	SERVER_CODE = `
type IServer interface {
	EntityByteSize(ruid.RUID, ruid.RUID, string) int
	EntitySerialize(ruid.RUID, ruid.RUID, string, *tygo.ProtoBuf)
	EntityDeserialize(*tygo.ProtoBuf) (ruid.RUID, ruid.RUID, string, error)
	EntityMessage(ruid.RUID, ruid.RUID, string, string, string) error
	ServiceProcedure(ruid.RUID, ruid.RUID, string, string) (string, error)
}

var Server IServer
`
	ENTITY_CODE = `
type Entity struct {
	tygo.Tygo
	ruid.RUID
	Key  ruid.RUID
	Type string
}

func (e *Entity) ByteSize() (size int) {
	if e != nil {
		size = Server.EntityByteSize(e.RUID, e.Key, e.Type)
		e.SetCachedSize(size)
	}
	return
}

func (e *Entity) Serialize(output *tygo.ProtoBuf) {
	if e != nil {
		Server.EntitySerialize(e.RUID, e.Key, e.Type, output)
	}
}

func (e *Entity) Deserialize(input *tygo.ProtoBuf) (err error) {
	e.RUID, e.Key, e.Type, err = Server.EntityDeserialize(input)
	return
}
`
)

func Entity(path string, types []tygo.Type) {
	pkgname, depends := Extract(path)
	pkgs := ENTITY_PKG
	var head bytes.Buffer
	var body bytes.Buffer
	head.Write([]byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

package %s
`, pkgname)))
	body.Write([]byte(`
`))
	body.Write([]byte(SERVER_CODE))
	body.Write([]byte(ENTITY_CODE))

	var services []*tygo.Object
	for _, t := range types {
		if object, ok := isService(t); ok {
			services = append(services, object)
		}
	}

	for _, depend := range depends {
		types = tygo.Extract(depend.Path, nil)
		for _, t := range types {
			for _, s := range depend.Services {
				if object, ok := isService(t); ok {
					if s == object.Name {
						services = append(services, object)
						srv_s, srv_p := injectService(object, false)
						body.Write([]byte(srv_s))
						pkgs = update(pkgs, srv_p)
					}
				}
			}
		}
	}
	msg_s, msg_p := entityMessage(services)
	body.Write([]byte(msg_s))
	pkgs = update(pkgs, msg_p)

	var sortedPkg []string
	for path, _ := range pkgs {
		sortedPkg = append(sortedPkg, path)
	}
	sort.Strings(sortedPkg)
	for _, path := range sortedPkg {
		head.Write([]byte(fmt.Sprintf(`
import %s"%s"`, pkgs[path], path)))
	}

	head.Write(body.Bytes())
	ioutil.WriteFile(SRC_PATH+path+"/entity.rpc.go", head.Bytes(), 0666)
}

func entityMessage(services []*tygo.Object) (string, map[string]string) {
	for _, s := range services {
		fmt.Println("entityMessage", s.Name)
	}
	return "", nil
}
