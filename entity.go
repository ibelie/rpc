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

var ENTITY_PKG = map[string]string{
	"fmt": "",
	"github.com/ibelie/rpc":  "",
	"github.com/ibelie/ruid": "",
	"github.com/ibelie/tygo": "",
}

const (
	ENTITY_CODE = `
var Server  rpc.Server

type Entity struct {
	tygo.Tygo
	ruid.ID
	Key  ruid.ID
	Type string
}

type ClientDelegate struct {
	e *Entity
	s *Entity
}

func (e *Entity) Client(session *Entity) *ClientDelegate {
	return &ClientDelegate{e: e, s: session}
}

func (e *Entity) Create() (err error) {
	size := 1 + tygo.SymbolEncodedLen(e.Type)
	if e.Key.Nonzero() {
		size += e.Key.ByteSize()
	}
	data := make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	if e.Key.Nonzero() {
		output.WriteBytes(1)
		e.Key.Serialize(output)
	} else {
		output.WriteBytes(0)
	}
	output.EncodeSymbol(e.Type)
	_, err = Server.Distribute(e.ID, e.Key, e.Type, SYMBOL_CREATE, data)
	return
}

func (e *Entity) Destroy() (err error) {
	_, err = Server.Distribute(e.ID, e.Key, e.Type, SYMBOL_DESTROY, nil)
	return
}

func (e *Entity) ByteSize() (size int) {
	if e != nil {
		size = 1 + tygo.SymbolEncodedLen(e.Type)
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
		var t byte
		if e.ID.Nonzero() {
			t |= 1
		}
		if e.Key.Nonzero() {
			t |= 2
		}
		output.WriteBytes(t)
		if e.ID.Nonzero() {
			e.ID.Serialize(output)
		}
		if e.Key.Nonzero() {
			e.Key.Serialize(output)
		}
		output.EncodeSymbol(e.Type)
	}
}

func (e *Entity) Deserialize(input *tygo.ProtoBuf) (err error) {
	var t byte
	if t, err = input.ReadByte(); err != nil {
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
	e.Type = tygo.DecodeSymbol(input.Bytes())
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
		if object, ok := isComponent(t); ok {
			services = append(services, object)
		}
	}

	for _, depend := range depends {
		for _, t := range tygo.Extract(depend.Path, nil) {
			for _, s := range depend.Services {
				if object, ok := isComponent(t); ok && s == object.Name {
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

	var symbols []string
	for _, s := range services {
		symbols = append(symbols, s.Name)
		for _, f := range s.VisibleFields() {
			symbols = append(symbols, f.Name)
		}
		for _, m := range s.Methods {
			symbols = append(symbols, m.Name)
		}
	}
	symbol_c, _ := entitySymbols(symbols)
	body.Write([]byte(symbol_c))

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

	var symbols []string
	for _, e := range entities {
		symbols = append(symbols, e.Name)
		symbols = append(symbols, fmt.Sprintf(CLIENT_ENTITY_FMT, e.Name))
		for _, c := range e.Components {
			if c.Protocol == nil {
				continue
			}
			symbols = append(symbols, c.Name)
			for _, f := range c.Protocol.VisibleFields() {
				symbols = append(symbols, f.Name)
			}
			for _, m := range c.Protocol.Methods {
				symbols = append(symbols, m.Name)
			}
		}
	}
	symbol_c, symbol_v := entitySymbols(symbols)

	ioutil.WriteFile(path.Join(dir, "proxy.go"), []byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

package main

import %q

var Ident = %s
%s
%s
%s
`, ident[0], ident[1], symbol_c, symbol_v, entityRoutes(entities))), 0666)
}

func entityRoutes(entities []*Entity) string {
	var routes []string
	methodMap := make(map[string][]string)
	componentMap := make(map[string]*Component)
	for _, e := range entities {
		var components []string
		pMethods := make(map[string]bool)
		for _, c := range e.Components {
			if c.Protocol == nil {
				continue
			}
			for _, m := range c.Protocol.Methods {
				pMethods[m.Name] = true
			}
			if c.isService() {
				components = append(components, fmt.Sprintf(`
		SYMBOL_%s: true,`, c.Name))
			}
			componentMap[c.Name] = c
		}

		bMethods := make(map[string]bool)
		for _, b := range e.Behaviors {
			for _, m := range b.Methods {
				bMethods[m] = true
			}
		}

		for m, ok := range pMethods {
			if ok {
				if ok, exist := bMethods[m]; ok && exist {
					methodMap[m] = append(methodMap[m], fmt.Sprintf(CLIENT_ENTITY_FMT, e.Name))
				}
			}
		}

		routes = append(routes, fmt.Sprintf(`
	SYMBOL_%s: map[string]bool{%s
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
	SYMBOL_%s: map[string]bool{%s
	},`, m, strings.Join(components, "")))
	}

	return fmt.Sprintf(`
var Routes = map[string]map[string]bool{%s
}`, strings.Join(routes, ""))
}

func entitySymbols(symbols []string) (string, string) {
	maxSymbol := 0
	symbolMap := make(map[string]bool)
	for _, s := range BUILTIN_SYMBOLS {
		if maxSymbol < len(s) {
			maxSymbol = len(s)
		}
		symbolMap[s] = false
	}

	for _, s := range symbols {
		if maxSymbol < len(s) {
			maxSymbol = len(s)
		}
		symbolMap[s] = true
	}

	var symbolSorted []string
	for s, ok := range symbolMap {
		if ok {
			symbolSorted = append(symbolSorted, s)
		}
	}
	sort.Strings(symbolSorted)

	var symbolConst []string
	var symbolValue []string
	for _, s := range BUILTIN_SYMBOLS {
		symbolConst = append(symbolConst, fmt.Sprintf(`
	SYMBOL_%s %s= %q`, s, strings.Repeat(" ", maxSymbol-len(s)), s))
		symbolValue = append(symbolValue, fmt.Sprintf(`
	SYMBOL_%s,`, s))
	}
	for _, s := range symbolSorted {
		symbolConst = append(symbolConst, fmt.Sprintf(`
	SYMBOL_%s %s= %q`, s, strings.Repeat(" ", maxSymbol-len(s)), s))
		symbolValue = append(symbolValue, fmt.Sprintf(`
	SYMBOL_%s,`, s))
	}

	return fmt.Sprintf(`
const (%s
)`, strings.Join(symbolConst, "")), fmt.Sprintf(`
var Symbols = []string{%s
}`, strings.Join(symbolValue, ""))
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
			err = fmt.Errorf("[%s] Distribute '%s' errors:%%s", strings.Join(errors, ""))
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

	pkgs = update(pkgs, FMT_PKG)
	return fmt.Sprintf(`
func (e *Entity) %s(%s) (%serr error) {
	if e == nil {
		err = fmt.Errorf("[Entity] Distribute '%s' is called on nil entity")
		return
	}%s
	return
}

func (c *ClientDelegate) %s(%s) (%serr error) {
	if c == nil || c.s == nil || c.e == nil {
		err = fmt.Errorf("[Entity] Message '%s' is called on nil client")
		return
	}%s
	return
}
`, method.Name, strings.Join(params_declare, ", "), dist_declare, method.Name, dist_result,
		method.Name, strings.Join(params_declare, ", "), proc_declare, method.Name, proc_result), pkgs
}
