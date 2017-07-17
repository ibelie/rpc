// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"io/ioutil"

	"github.com/ibelie/tygo"
)

var (
	ENTITY_PKG = map[string]string{
		"github.com/ibelie/ruid": "",
		"github.com/ibelie/tygo": "",
	}
	builtinSymbols = []string{
		"CREATE",
		"DESTROY",
		"SYNCHRON",
		"NOTIFY",
	}
)

const (
	SERVER_CODE = `
type IServer interface {
	Distribute(ruid.RUID, ruid.RUID, uint64, uint64, []byte, chan<- []byte) error
	Procedure(ruid.RUID, ruid.RUID, uint64, uint64, []byte) ([]byte, error)
}

var Server IServer
var Symbols map[string]uint64
`
	ENTITY_CODE = `
type Entity struct {
	tygo.Tygo
	ruid.RUID
	Key  ruid.RUID
	Type uint64
}

func NewEntity(i ruid.RUID, k ruid.RUID, t string) *Entity {
	return &Entity{RUID: i, Key: k, Type: Symbols[t]}
}

func (e *Entity) Create() error {
	data := make([]byte, tygo.SizeVarint(e.Key) + tygo.SizeVarint(e.Type))
	output := &tygo.ProtoBuf{Buffer: data}
	output.WriteVarint(e.Key)
	output.WriteVarint(e.Type)
	return Server.Distribute(e.RUID, e.Key, e.Type, SYMBOL_CREATE, data, nil)
}

func (e *Entity) Destroy() error {
	return Server.Distribute(e.RUID, e.Key, e.Type, SYMBOL_DESTROY, nil, nil)
}

func (e *Entity) ByteSize() (size int) {
	if e != nil {
		size = tygo.SizeVarint(e.RUID) + tygo.SizeVarint(e.Key) + tygo.SizeVarint(e.Type)
		e.SetCachedSize(size)
	}
	return
}

func (e *Entity) Serialize(output *tygo.ProtoBuf) {
	if e != nil {
		output.WriteVarint(e.RUID)
		output.WriteVarint(e.Key)
		output.WriteVarint(e.Type)
	}
}

func (e *Entity) Deserialize(input *tygo.ProtoBuf) (err error) {
	var i, k uint64
	if i, err = input.ReadVarint(); err != nil {
		return
	} else if k, err = input.ReadVarint(); err != nil {
		return
	} else if e.Type, err = input.ReadVarint(); err != nil {
		return
	}
	e.RUID = i
	e.Key = k
	return
}
`
)

func Go(dir string, types []tygo.Type) {
	pkgname, depends := Extract(dir)
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
		for _, t := range tygo.Extract(depend.Path, nil) {
			for _, s := range depend.Services {
				if object, ok := isService(t); ok {
					if s == object.Name {
						services = append(services, object)
						srv_s, srv_p := injectServiceCommon(object, "")
						body.Write([]byte(srv_s))
						pkgs = update(pkgs, srv_p)
					}
				}
			}
		}
	}

	init_s, init_p := entityInitialize(services)
	body.Write([]byte(init_s))
	pkgs = update(pkgs, init_p)

	methodRecord := make(map[string]bool)
	for _, s := range services {
		for _, m := range s.Methods {
			if ok, exist := methodRecord[m.Name]; exist && ok {
				continue
			} else if len(m.Results) > 1 {
				continue
			}
			method_s, method_p := entityDistribute(s, m)
			body.Write([]byte(method_s))
			pkgs = update(pkgs, method_p)
			methodRecord[m.Name] = true
		}
	}

	var sortedPkg []string
	for pkg, _ := range pkgs {
		sortedPkg = append(sortedPkg, pkg)
	}
	sort.Strings(sortedPkg)
	for _, pkg := range sortedPkg {
		head.Write([]byte(fmt.Sprintf(`
import %s"%s"`, pkgs[pkg], pkg)))
	}

	head.Write(body.Bytes())
	ioutil.WriteFile(path.Join(SRC_PATH, dir, "entity.rpc.go"), head.Bytes(), 0666)
}

func Tables(dir string, entities []*Entity) {
	maxSymbol := 0
	symbolMap := make(map[string]bool)
	for _, e := range entities {
		symbolMap[e.Name] = true
		for _, c := range e.Components {
			symbolMap[c.Name] = true
			for _, f := range c.Fields {
				symbolMap[f.Name] = true
			}
			for _, m := range c.Methods {
				symbolMap[m.Name] = true
			}
		}
	}

	var symbolConsts []string
	var symbolValues []string
	var symbolSorted []string
	for i, s := range builtinSymbols {
		if maxSymbol < len(s) {
			maxSymbol = len(s)
		}
		if i == 0 {
			symbolConsts = append(symbolConsts, fmt.Sprintf(`
	SYMBOL_%s uint64 = iota`, s))
		} else {
			symbolConsts = append(symbolConsts, fmt.Sprintf(`
	SYMBOL_%s`, s))
		}
	}

	for s, ok := range symbolMap {
		if !ok {
			continue
		}
		if maxSymbol < len(s) {
			maxSymbol = len(s)
		}
		symbolSorted = append(symbolSorted, s)
	}
	sort.Strings(symbolSorted)

	for _, s := range builtinSymbols {
		symbolValues = append(symbolValues, fmt.Sprintf(`
	"%s": %sSYMBOL_%s,`, s, strings.Repeat(" ", maxSymbol-len(s)), s))
	}
	for _, s := range symbolSorted {
		symbolConsts = append(symbolConsts, fmt.Sprintf(`
	SYMBOL_%s`, s))
		symbolValues = append(symbolValues, fmt.Sprintf(`
	"%s": %sSYMBOL_%s,`, s, strings.Repeat(" ", maxSymbol-len(s)), s))
	}

	ioutil.WriteFile(path.Join(dir, "symbol.tbl.go"), []byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

package main

const (%s
)

var Symbols = map[string]uint64{%s
}
`, strings.Join(symbolConsts, ""), strings.Join(symbolValues, ""))), 0666)

	ioutil.WriteFile(path.Join(dir, "route.tbl.go"), []byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

package main
`)), 0666)
}

func entityInitialize(services []*tygo.Object) (string, map[string]string) {
	symbol_set := make(map[string]bool)
	var symbol_declare []string
	var symbol_initialize []string
	for _, service := range services {
		if ok, exist := symbol_set[service.Name]; !ok || !exist {
			symbol_declare = append(symbol_declare, fmt.Sprintf(`
	SYMBOL_%s uint64`, service.Name))
			symbol_initialize = append(symbol_initialize, fmt.Sprintf(`
		SYMBOL_%s = Symbols["%s"]`, service.Name, service.Name))
			symbol_set[service.Name] = true
		}
		for _, field := range service.Fields {
			if ok, exist := symbol_set[field.Name]; !ok || !exist {
				symbol_declare = append(symbol_declare, fmt.Sprintf(`
	SYMBOL_%s uint64`, field.Name))
				symbol_initialize = append(symbol_initialize, fmt.Sprintf(`
		SYMBOL_%s = Symbols["%s"]`, field.Name, field.Name))
				symbol_set[field.Name] = true
			}
		}
		for _, method := range service.Methods {
			if ok, exist := symbol_set[method.Name]; !ok || !exist {
				symbol_declare = append(symbol_declare, fmt.Sprintf(`
	SYMBOL_%s uint64`, method.Name))
				symbol_initialize = append(symbol_initialize, fmt.Sprintf(`
		SYMBOL_%s = Symbols["%s"]`, method.Name, method.Name))
				symbol_set[method.Name] = true
			}
		}
	}

	var symbol_const []string
	for i, s := range builtinSymbols {
		if i == 0 {
			symbol_const = append(symbol_const, fmt.Sprintf(`
	SYMBOL_%s uint64 = iota`, s))
		} else {
			symbol_const = append(symbol_const, fmt.Sprintf(`
	SYMBOL_%s`, s))
		}
	}

	return fmt.Sprintf(`
const (%s
)

var (%s
)

func InitializeServer(server IServer, symbols map[string]uint64) {
	if Server == nil {
		Server = server
	}
	if Symbols == nil {
		Symbols = symbols%s
	}
}
`, strings.Join(symbol_const, ""), strings.Join(symbol_declare, ""), strings.Join(symbol_initialize, "")), nil
}

func entityDistribute(service *tygo.Object, method *tygo.Method) (string, map[string]string) {
	var pkgs map[string]string
	var param string
	var params_list []string
	var params_declare []string
	for i, p := range method.Params {
		param_s, param_p := p.Go()
		pkgs = update(pkgs, param_p)
		params_list = append(params_list, fmt.Sprintf("p%d", i))
		params_declare = append(params_declare, fmt.Sprintf("p%d %s", i, param_s))
	}

	if len(params_list) > 0 {
		param = fmt.Sprintf("(*%sDelegate)(nil).Serialize%sParam(%s)",
			service.Name, method.Name, strings.Join(params_list, ", "))
	} else {
		param = "nil"
	}

	var result string
	if len(method.Results) == 1 {
		result_s, result_p := method.Results[0].Go()
		pkgs = update(pkgs, result_p)
		params_declare = append(params_declare, fmt.Sprintf("rChan chan<- %s", result_s))
		result = fmt.Sprintf(`
	if rChan != nil {
		var err error
		resultChan := make(chan []byte)
		go func() {
			if err = Server.Distribute(e.RUID, e.Key, e.Type, SYMBOL_%s, %s, resultChan); err == nil {
				for _, result := range resultChan {
					var r %s
					r, err = (*%sDelegate)(nil).Deserialize%sResult(result)
					rChan <- r
				}
			}
			close(rChan)
		}()
		return err
	} else {
		return Server.Distribute(e.RUID, e.Key, e.Type, SYMBOL_%s, %s, nil)
	}`, method.Name, param, result_s, service.Name, method.Name, method.Name, param)
	} else {
		result = fmt.Sprintf(`
	return Server.Distribute(e.RUID, e.Key, e.Type, SYMBOL_%s, %s, nil)`,
			method.Name, param)
	}

	return fmt.Sprintf(`
func (e *Entity) %s(%s) error {%s
}
`, method.Name, strings.Join(params_declare, ", "), result), pkgs
}
