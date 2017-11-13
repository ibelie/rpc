// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"fmt"
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

var (
	SRC_PATH = path.Join(os.Getenv("GOPATH"), "src")
	PKG_PATH = reflect.TypeOf(Depend{}).PkgPath()
)

type Depend struct {
	Path     string
	Services []string
}

type Component struct {
	Name     string
	Path     string
	Protocol *tygo.Object
	Service  *doc.Type
}

type Behavior struct {
	Name       string
	Methods    []string
	Components []*Component
}

type Entity struct {
	Name       string
	Behavior   []*Behavior
	Components []*Component
}

func Extract(dir string) (pkgname string, depends []*Depend) {
	buildPackage, err := build.Import(dir, "", build.ImportComment)
	if err != nil {
		log.Fatalf("[RPC][Entity] Cannot import package:\n>>>> %v", err)
		return
	}
	fs := token.NewFileSet()
	for _, filename := range buildPackage.GoFiles {
		file, err := parser.ParseFile(fs, path.Join(buildPackage.Dir, filename), nil, parser.ParseComments)
		if err != nil {
			log.Fatalf("[RPC][Entity] Cannot parse file:\n>>>> %v", err)
		}
		pkgname = file.Name.Name
	file_loop:
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
				break file_loop
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
		a = make(map[string]string)
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

func isComponent(t tygo.Type) (*tygo.Object, bool) {
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

func (c *Component) isService() bool {
	if hasMethod(c.Service, &tygo.Method{Name: "onCreate"}) {
		return true
	} else if hasMethod(c.Service, &tygo.Method{Name: "onDestroy"}) {
		return true
	}
	for _, m := range c.Protocol.Methods {
		if hasMethod(c.Service, m) {
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

func resolveTypes(typesMap map[string]tygo.Type, typ tygo.Type) {
	switch t := typ.(type) {
	case *tygo.InstanceType:
		if _, exist := typesMap[t.Name]; !exist && (t.Name != "Entity" || t.PkgName != "" || t.PkgPath != "") {
			for _, typ := range tygo.Extract(t.PkgPath, ReplaceEntity) {
				switch t := typ.(type) {
				case *tygo.Object:
					typesMap[t.Name] = t
					resolveTypes(typesMap, t)
				case *tygo.Enum:
					typesMap[t.Name] = t
					resolveTypes(typesMap, t)
				}
			}
			if _, exist := typesMap[t.Name]; !exist {
				log.Fatalf("[RPC][Entity] Cannot resolve type: %v", t)
			}
		}
	case *tygo.Object:
		if t.HasParent() {
			resolveTypes(typesMap, t.Parent)
		}
		for _, f := range t.Fields {
			resolveTypes(typesMap, f)
		}
		for _, m := range t.Methods {
			for _, p := range m.Params {
				resolveTypes(typesMap, p)
			}
			for _, r := range m.Results {
				resolveTypes(typesMap, r)
			}
		}
	case *tygo.ListType:
		resolveTypes(typesMap, t.E)
	case *tygo.DictType:
		resolveTypes(typesMap, t.K)
		resolveTypes(typesMap, t.V)
	case *tygo.VariantType:
		for _, ts := range t.Ts {
			resolveTypes(typesMap, ts)
		}
	}
}

func resolveEntities(entities []*Entity) (types []tygo.Type) {
	fs := token.NewFileSet()
	pkgMap := make(map[string][]tygo.Type)
	docMap := make(map[string]map[string]*doc.Type)
	typesMap := make(map[string]tygo.Type)
	for _, e := range entities {
		for _, c := range e.Components {
			ts, ok := pkgMap[c.Path]
			if !ok {
				if p, err := build.Import(c.Path, "", build.ImportComment); err != nil {
					if strings.Contains(err.Error(), fmt.Sprintf("cannot find package %q in any of:\n", c.Path)) {
						log.Printf("[RPC][Entity] Ignore client component %q from %q", c.Name, c.Path)
					} else {
						log.Fatalf("[RPC][Entity] Import component %q from %q:\n>>>> %v", c.Name, c.Path, err)
					}
					continue
				} else {
					include := func(info os.FileInfo) bool {
						for _, name := range p.GoFiles {
							if name == info.Name() {
								return true
							}
						}
						return false
					}
					if pkgs, err := parser.ParseDir(fs, p.Dir, include, parser.ParseComments); err == nil && len(pkgs) == 1 {
						docTypes := make(map[string]*doc.Type)
						for _, t := range doc.New(pkgs[p.Name], p.ImportPath, doc.AllDecls).Types {
							docTypes[t.Name] = t
						}
						docMap[c.Path] = docTypes
					}
					ts = tygo.Extract(c.Path, ReplaceEntity)
					pkgMap[c.Path] = ts
				}
			}
			c.Service = docMap[c.Path][c.Name]
			for _, typ := range ts {
				switch t := typ.(type) {
				case *tygo.Object:
					if t.Name == c.Name {
						c.Protocol = t
					}
					typesMap[t.Name] = t
				case *tygo.Enum:
					typesMap[t.Name] = t
				}
			}
		}
	}

	var typesSorted []string
	for n, _ := range typesMap {
		typesSorted = append(typesSorted, n)
	}
	for _, n := range typesSorted {
		resolveTypes(typesMap, typesMap[n])
	}
	typesSorted = nil
	for n, _ := range typesMap {
		typesSorted = append(typesSorted, n)
	}
	sort.Strings(typesSorted)
	for _, n := range typesSorted {
		types = append(types, typesMap[n])
	}

	return
}
