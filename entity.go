// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"io/ioutil"

	"github.com/ibelie/tygo"
)

const (
	IDENT_RUID = iota
	IDENT_UUID
	IDENT_STRID
)

var IDENT_PKG = [][2]string{
	[2]string{"github.com/ibelie/ruid", "ruid.RUIdent"},
	[2]string{"github.com/ibelie/rpc/uuid", "uuid.UUIdent"},
	[2]string{"github.com/ibelie/rpc/strid", "strid.STRIdent"},
}

func IDENT_FromString(name string) int {
	switch strings.ToUpper(name) {
	case "RUID":
		return IDENT_RUID
	case "UUID":
		return IDENT_UUID
	case "STRID":
		return IDENT_STRID
	default:
		log.Fatalf("[RPC][Identifier] Unknown identifier name: %q", name)
		return -1
	}
}

var (
	RPC_PKG    = map[string]string{"github.com/ibelie/rpc": ""}
	ENTITY_PKG = map[string]string{
		"github.com/ibelie/rpc":  "",
		"github.com/ibelie/ruid": "",
		"github.com/ibelie/tygo": "",
	}
)

const (
	ENTITY_CODE = `
var (
	Server  rpc.Server
	Symbols map[string]uint64
)

type Entity struct {
	tygo.Tygo
	ruid.ID
	Key  ruid.ID
	Type uint64
}

type ClientDelegate struct {
	e *Entity
	s *Entity
}

func NewEntity(i ruid.ID, k ruid.ID, t string) *Entity {
	return &Entity{ID: i, Key: k, Type: Symbols[t]}
}

func (e *Entity) Client(session *Entity) *ClientDelegate {
	return &ClientDelegate{e: e, s: session}
}

func (e *Entity) Create() (err error) {
	t := e.Type << 1
	size := tygo.SizeVarint(t)
	if e.Key.Nonzero() {
		size += e.Key.ByteSize()
		t |= 1
	}
	data := make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	output.WriteVarint(t)
	if e.Key.Nonzero() {
		e.Key.Serialize(output)
	}
	_, err = Server.Distribute(e.ID, e.Key, e.Type, SYMBOL_CREATE, data)
	return
}

func (e *Entity) Destroy() (err error) {
	_, err = Server.Distribute(e.ID, e.Key, e.Type, SYMBOL_DESTROY, nil)
	return
}

func (e *Entity) ByteSize() (size int) {
	if e != nil {
		size = tygo.SizeVarint(e.Type << 2)
		if e.ID.Nonzero() {
			size += e.ID.ByteSize()
		}
		if e.Key.Nonzero() {
			size += e.Key.ByteSize()
		}
		e.SetCachedSize(size)
	}
	return
}

func (e *Entity) Serialize(output *tygo.ProtoBuf) {
	if e != nil {
		t := e.Type << 2
		if e.ID.Nonzero() {
			t |= 1
		}
		if e.Key.Nonzero() {
			t |= 2
		}
		output.WriteVarint(t)
		if e.ID.Nonzero() {
			e.ID.Serialize(output)
		}
		if e.Key.Nonzero() {
			e.Key.Serialize(output)
		}
	}
}

func (e *Entity) Deserialize(input *tygo.ProtoBuf) (err error) {
	var t uint64
	if t, err = input.ReadVarint(); err != nil {
		return
	}
	if t&1 == 0 {
		e.ID = Server.ZeroID()
	} else if e.ID, err = Server.DeserializeID(input); err != nil {
		return
	}
	if t&2 == 0 {
		e.Key = Server.ZeroID()
	} else if e.Key, err = Server.DeserializeID(input); err != nil {
		return
	}
	e.Type = t >> 2
	return
}
`
)

func Go(dir string, types []tygo.Type) {
	pkgname, depends := Extract(dir)
	var head bytes.Buffer
	var body bytes.Buffer
	var pkgs map[string]string
	pkgs = update(pkgs, ENTITY_PKG)
	head.Write([]byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

package %s
`, pkgname)))
	body.Write([]byte(`
`))
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
				if object, ok := isService(t); ok && s == object.Name {
					var methods []string
					services = append(services, object)
					for _, method := range object.Methods {
						if len(method.Params) > 0 {
							param_s, param_p := tygo.TypeListSerialize(object.Name+DELEGATE, method.Name, "param", method.Params)
							pkgs = update(pkgs, param_p)
							methods = append(methods, param_s)
						}
						if len(method.Results) > 0 {
							result_s, result_p := tygo.TypeListDeserialize(object.Name+DELEGATE, method.Name, "result", method.Results)
							pkgs = update(pkgs, result_p)
							methods = append(methods, result_s)
						}
						method_s, method_p := injectProcedureCaller(object.Name+DELEGATE, object.Name, method, "")
						pkgs = update(pkgs, method_p)
						methods = append(methods, method_s)
					}

					body.Write([]byte(fmt.Sprintf(`
type %sDelegate Entity

func (e *Entity) %s() *%sDelegate {
	return (*%sDelegate)(e)
}
%s`, object.Name, object.Name, object.Name, object.Name, strings.Join(methods, ""))))
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
import %s%q`, pkgs[pkg], pkg)))
	}

	head.Write(body.Bytes())
	ioutil.WriteFile(path.Join(SRC_PATH, dir, "entity.rpc.go"), head.Bytes(), 0666)
}

func Proxy(identName string, dir string, entities []*Entity) {
	ident := IDENT_PKG[IDENT_FromString(identName)]
	ioutil.WriteFile(path.Join(dir, "proxy.go"), []byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

package main

import %q

var Ident = %s
%s%s`, ident[0], ident[1], proxySymbols(entities), proxyRoutes(entities))), 0666)
}

func proxySymbols(entities []*Entity) string {
	maxSymbol := 0
	symbolMap := make(map[string]bool)
	for _, e := range entities {
		symbolMap[e.Name] = true
		for _, c := range e.Components {
			if c.Protocol == nil {
				continue
			}
			symbolMap[c.Name] = true
			for _, f := range c.Protocol.VisibleFields() {
				symbolMap[f.Name] = true
			}
			for _, m := range c.Protocol.Methods {
				symbolMap[m.Name] = true
			}
		}
	}

	var symbolConsts []string
	for i, s := range BUILTIN_SYMBOLS {
		if maxSymbol < len(s) {
			maxSymbol = len(s)
		}
		if ok, exist := symbolMap[s]; exist && ok {
			delete(symbolMap, s)
		}
		if i == 0 {
			symbolConsts = append(symbolConsts, fmt.Sprintf(`
	SYMBOL_%s uint64 = iota`, s))
		} else {
			symbolConsts = append(symbolConsts, fmt.Sprintf(`
	SYMBOL_%s`, s))
		}
	}

	var symbolSorted []string
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

	var symbolValues []string
	for _, s := range BUILTIN_SYMBOLS {
		symbolValues = append(symbolValues, fmt.Sprintf(`
	%q: %sSYMBOL_%s,`, s, strings.Repeat(" ", maxSymbol-len(s)), s))
	}
	for _, s := range symbolSorted {
		symbolConsts = append(symbolConsts, fmt.Sprintf(`
	SYMBOL_%s`, s))
		symbolValues = append(symbolValues, fmt.Sprintf(`
	%q: %sSYMBOL_%s,`, s, strings.Repeat(" ", maxSymbol-len(s)), s))
	}

	return fmt.Sprintf(`
const (%s
)

var Symbols = map[string]uint64{%s
}
`, strings.Join(symbolConsts, ""), strings.Join(symbolValues, ""))
}

func proxyRoutes(entities []*Entity) string {
	var routes []string
	methodMap := make(map[string][]string)
	componentMap := make(map[string]*Component)
	for _, e := range entities {
		var components []string
		cMethods := make(map[string]bool)
		pMethods := make(map[string]bool)
		for _, c := range e.Components {
			if c.Protocol == nil {
				continue
			}
			for _, m := range c.Protocol.Methods {
				pMethods[m.Name] = true
			}
			for _, m := range c.Methods {
				cMethods[m] = true
			}
			components = append(components, fmt.Sprintf(`
		SYMBOL_%s: true,`, c.Name))
			componentMap[c.Name] = c
		}

		for m, ok := range pMethods {
			if ok {
				if ok, exist := cMethods[m]; ok && exist {
					methodMap[m] = append(methodMap[m], e.Name)
				}
			}
		}

		routes = append(routes, fmt.Sprintf(`
	SYMBOL_%s: map[uint64]bool{%s
	},`, e.Name, strings.Join(components, "")))
	}

	for _, c := range componentMap {
		for _, p := range c.Protocol.Methods {
			for _, m := range c.Service.Methods {
				if p.Name == m.Name {
					methodMap[p.Name] = append(methodMap[p.Name], c.Name)
					break
				}
			}
		}
	}

	var methodSorted []string
	for m, r := range methodMap {
		if len(r) > 0 {
			methodSorted = append(methodSorted, m)
		}
		sort.Strings(r)
	}
	sort.Strings(methodSorted)
	for _, m := range methodSorted {
		var components []string
		for _, c := range methodMap[m] {
			components = append(components, fmt.Sprintf(`
		SYMBOL_%s: true,`, c))
		}
		routes = append(routes, fmt.Sprintf(`
	SYMBOL_%s: map[uint64]bool{%s
	},`, m, strings.Join(components, "")))
	}

	return fmt.Sprintf(`
var Routes = map[uint64]map[uint64]bool{%s
}
`, strings.Join(routes, ""))
}

func entityInitialize(services []*tygo.Object) (string, map[string]string) {
	maxSymbol := 0
	symbolMap := make(map[string]bool)
	for _, s := range services {
		symbolMap[s.Name] = true
		for _, f := range s.VisibleFields() {
			symbolMap[f.Name] = true
		}
		for _, m := range s.Methods {
			symbolMap[m.Name] = true
		}
	}

	var symbolSorted []string
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

	var symbolDeclare []string
	var symbolInitialize []string
	for _, s := range symbolSorted {
		symbolDeclare = append(symbolDeclare, fmt.Sprintf(`
	SYMBOL_%s %suint64`, s, strings.Repeat(" ", maxSymbol-len(s))))
		symbolInitialize = append(symbolInitialize, fmt.Sprintf(`
		SYMBOL_%s = Symbols[%q]`, s, s))
	}

	var symbolConst []string
	for i, s := range BUILTIN_SYMBOLS {
		if i == 0 {
			symbolConst = append(symbolConst, fmt.Sprintf(`
	SYMBOL_%s uint64 = iota`, s))
		} else {
			symbolConst = append(symbolConst, fmt.Sprintf(`
	SYMBOL_%s`, s))
		}
	}

	return fmt.Sprintf(`
const (%s
)

var (%s
)

func InitializeServer(server rpc.Server, symbols map[string]uint64) {
	if Server == nil {
		Server = server
	}
	if Symbols == nil {
		Symbols = symbols%s
	}
}
`, strings.Join(symbolConst, ""), strings.Join(symbolDeclare, ""),
		strings.Join(symbolInitialize, "")), RPC_PKG
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

	var dist_result string
	var dist_declare string
	var proc_result string
	var proc_declare string
	if len(method.Results) == 1 {
		result_s, result_p := method.Results[0].Go()
		pkgs = update(pkgs, result_p)
		pkgs = update(pkgs, FMT_PKG)
		pkgs = update(pkgs, STR_PKG)
		dist_declare = fmt.Sprintf("rs []%s, ", result_s)
		dist_result = fmt.Sprintf(`
	if results, er := Server.Distribute(e.ID, e.Key, e.Type, SYMBOL_%s, %s); er != nil {
		err = er
	} else {
		var errors []string
		for _, result := range results {
			if r, er := (*%sDelegate)(nil).Deserialize%sResult(result); er != nil {
				errors = append(errors, fmt.Sprintf("\n>>>> %%v", er))
			} else {
				rs = append(rs, r)
			}
		}
		if len(errors) > 0 {
			err = fmt.Errorf("[%s] %s errors:%%s", strings.Join(errors, ""))
		}
	}`, method.Name, param, service.Name, method.Name, service.Name, method.Name)
		proc_declare = fmt.Sprintf("r %s, ", result_s)
		proc_result = fmt.Sprintf(`
	result, err := Server.Message(c.s.ID, c.s.Key, c.e.ID, SYMBOL_%s, %s)
	if err == nil {
		r, err = (*%sDelegate)(nil).Deserialize%sResult(result)
	}`, method.Name, param, service.Name, method.Name)
	} else {
		dist_result = fmt.Sprintf(`
	_, err = Server.Distribute(e.ID, e.Key, e.Type, SYMBOL_%s, %s)`,
			method.Name, param)
		proc_result = fmt.Sprintf(`
	_, err = Server.Message(c.s.ID, c.s.Key, c.e.ID, SYMBOL_%s, %s)`,
			method.Name, param)
	}

	return fmt.Sprintf(`
func (e *Entity) %s(%s) (%serr error) {%s
	return
}

func (c *ClientDelegate) %s(%s) (%serr error) {%s
	return
}
`, method.Name, strings.Join(params_declare, ", "), dist_declare, dist_result,
		method.Name, strings.Join(params_declare, ", "), proc_declare, proc_result), pkgs
}
