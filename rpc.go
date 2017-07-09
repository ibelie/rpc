// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"log"
	"os"
	"reflect"
	"sort"
	"strings"

	"go/ast"
	"go/build"
	"go/parser"
	"go/token"

	"github.com/ibelie/tygo"
)

type Depend struct {
	Path     string
	Services []string
}

var (
	SRC_PATH = os.Getenv("GOPATH") + "/src/"
	PKG_PATH = reflect.TypeOf(Depend{}).PkgPath()
)

func Extract(path string) (pkgname string, depends []*Depend) {
	buildPackage, err := build.Import(path, "", build.ImportComment)
	if err != nil {
		log.Fatalf("[RPC][Entity] Cannot import package:\n>>>>%v", err)
		return
	}
	fs := token.NewFileSet()
	for _, filename := range buildPackage.GoFiles {
		file, err := parser.ParseFile(fs, buildPackage.Dir+"/"+filename, nil, parser.ParseComments)
		if err != nil {
			log.Fatalf("[RPC][Entity] Cannot parse file:\n>>>>%v", err)
		}
		pkgname = file.Name.Name
		for _, d := range file.Decls {
			decl, ok := d.(*ast.GenDecl)
			if !ok || decl.Tok != token.IMPORT {
				continue
			}
			for _, s := range decl.Specs {
				spec, ok := s.(*ast.ImportSpec)
				if !ok || strings.Trim(spec.Path.Value, "\"") != PKG_PATH {
					continue
				}
				if strings.TrimSpace(decl.Doc.Text()) != "" {
					depends = merge(depends, parse(decl.Doc.Text())...)
				}
			}
		}
	}
	return
}

func parse(code string) (depends []*Depend) {
	code = strings.Split(code, "depends on:")[1]
	for _, line := range strings.Split(code, "\n") {
		tokens := strings.Split(line, "from")
		if len(tokens) != 2 {
			continue
		}
		depends = merge(depends, &Depend{
			Services: []string{strings.TrimSpace(tokens[0])},
			Path:     strings.TrimSpace(tokens[1]),
		})
	}
	return
}

func merge(a []*Depend, b ...*Depend) (c []*Depend) {
	var sorted []string
	m := make(map[string]map[string]bool)
	for _, ab := range [][]*Depend{a, b} {
		for _, x := range ab {
			if _, ok := m[x.Path]; !ok {
				m[x.Path] = make(map[string]bool)
				sorted = append(sorted, x.Path)
			}
			for _, y := range x.Services {
				m[x.Path][y] = true
			}
		}
	}
	sort.Strings(sorted)
	for _, p := range sorted {
		var s []string
		for x, _ := range m[p] {
			s = append(s, x)
		}
		sort.Strings(s)
		c = append(c, &Depend{Path: p, Services: s})
	}
	return
}

func update(a map[string]string, b map[string]string) map[string]string {
	if b == nil {
		return a
	} else if a == nil {
		return b
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

func isService(t tygo.Type) (*tygo.Object, bool) {
	object, ok := t.(*tygo.Object)
	return object, ok && object.Parent.Name == "Entity"
}
