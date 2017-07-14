// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"log"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"

	"go/ast"
	"go/build"
	"go/doc"
	"go/parser"
	"go/token"

	"github.com/ibelie/tygo"
)

type Depend struct {
	Path     string
	Services []string
}

type Entity struct {
	Name       string
	Components []*tygo.Object
}

var (
	SRC_PATH = path.Join(os.Getenv("GOPATH"), "src")
	PKG_PATH = reflect.TypeOf(Depend{}).PkgPath()
)

func Extract(dir string) (pkgname string, depends []*Depend) {
	buildPackage, err := build.Import(dir, "", build.ImportComment)
	if err != nil {
		log.Fatalf("[RPC][Entity] Cannot import package:\n>>>>%v", err)
		return
	}
	fs := token.NewFileSet()
	for _, filename := range buildPackage.GoFiles {
		file, err := parser.ParseFile(fs, path.Join(buildPackage.Dir, filename), nil, parser.ParseComments)
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

func hasMethod(object *doc.Type, method *tygo.Method) bool {
	for _, m := range object.Methods {
		if m.Name == method.Name {
			return true
		}
	}
	return false
}

func packageDoc(path string) *doc.Package {
	p, err := build.Import(path, "", build.ImportComment)
	if err != nil {
		return nil
	}
	fs := token.NewFileSet()
	include := func(info os.FileInfo) bool {
		for _, name := range p.GoFiles {
			if name == info.Name() {
				return true
			}
		}
		return false
	}

	if pkgs, err := parser.ParseDir(fs, p.Dir, include, parser.ParseComments); err != nil || len(pkgs) != 1 {
		return nil
	} else {
		return doc.New(pkgs[p.Name], p.ImportPath, doc.AllDecls)
	}
}

func getEntities(entityMap map[string]map[string][]string) (entities []*Entity) {
	var entitySorted []string
	for n, _ := range entityMap {
		entitySorted = append(entitySorted, n)
	}
	sort.Strings(entitySorted)

	for _, n := range entitySorted {
		var componentSorted []string
		componentMap := make(map[string]*tygo.Object)
		for pkg, names := range entityMap[n] {
			if _, err := build.Import(pkg, "", build.ImportComment); err != nil {
				log.Printf("[RPC][Entity] Ignore component:\n>>>>%v", err)
				continue
			}
			for _, t := range tygo.Extract(pkg, nil) {
				for _, s := range names {
					if object, ok := t.(*tygo.Object); ok &&
						object.Parent.Name == "Entity" && s == object.Name {
						componentSorted = append(componentSorted, s)
						componentMap[s] = object
					}
				}
			}
		}
		sort.Strings(componentSorted)
		var components []*tygo.Object
		for _, c := range componentSorted {
			components = append(components, componentMap[c])
		}
		entities = append(entities, &Entity{Name: n, Components: components})
	}

	return
}
