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
	EntityType(string) int
	EntityName(int) string
}

var Server IServer
`
	ENTITY_CODE = `
type Entity struct {
	tygo.Tygo
	ruid.RUID
	Key  ruid.RUID
	Type string
	cachedSize int
}

func (e *Entity) MaxFieldNum() int {
	return 1
}

func (e *Entity) ByteSize() (size int) {
	if e != nil {
		e.cachedSize = tygo.SizeVarint(e.RUID) + tygo.SizeVarint(e.Key) + tygo.SizeVarint(Server.EntityType(e.Type))
		size = 1 + tygo.SizeVarint(e.cachedSize) + e.cachedSize
		e.SetCachedSize(size)
	}
	return
}

func (e *Entity) Serialize(output *tygo.ProtoBuf) {
	if e != nil {
		output.WriteBytes(10) // tag: 10 MAKE_TAG(1, WireBytes=2)
		output.WriteVarint(e.cachedSize)
		output.WriteVarint(e.RUID)
		output.WriteVarint(e.Key)
		output.WriteVarint(Server.EntityType(e.Type))
	}
}

func (e *Entity) Deserialize(input *tygo.ProtoBuf) (err error) {
	for !input.ExpectEnd() {
		var tag int
		var cutoff bool
		if tag, cutoff, err = input.ReadTag(127); err == nil && cutoff && tag == 10 { // MAKE_TAG(1, WireBytes=2)
			var buffer []byte
			if buffer, err = input.ReadBuf(); err == nil {
				var i, k, t uint64
				tmpi := &tygo.ProtoBuf{Buffer: buffer}
				if i, err = tmpi.ReadVarint(); err != nil {
					return
				} else if k, err = tmpi.ReadVarint(); err != nil {
					return
				} else if t, err = tmpi.ReadVarint(); err != nil {
					return
				}
				e.RUID = i
				e.Key  = k
				e.Type = Server.EntityName(t)
			}
			return
		} else if err = input.SkipField(tag); err != nil {
			return
		}
	}
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
						srv_s, srv_p := entityService(object)
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

func entityService(service *tygo.Object) (string, map[string]string) {
	fmt.Println("entityService", service.Name)
	return "", nil
}
