// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"log"
	"os"
	"reflect"
	"strings"

	"go/ast"
	"go/build"
	"go/parser"
	"go/token"

	_ "github.com/ibelie/tygo"
)

type Depend struct {
	Name string
	Path string
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
					depends = append(depends, Parse(decl.Doc.Text())...)
				}
			}
		}
	}
	return
}

func Parse(code string) (depends []*Depend) {
	code = strings.Split(code, "depends on:")[1]
	for _, line := range strings.Split(code, "\n") {
		tokens := strings.Split(line, "from")
		if len(tokens) != 2 {
			continue
		}
		depends = append(depends, &Depend{Name: strings.TrimSpace(tokens[0]), Path: strings.TrimSpace(tokens[1])})
	}
	return
}
