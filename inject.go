// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"io/ioutil"

	"github.com/ibelie/tygo"
)

var (
	LOCAL_PKG = map[string]string{
		"sync": "",
		"github.com/ibelie/ruid": "",
	}
)

func Inject(path string, filename string, pkgname string, types []tygo.Type) {
	var services []*tygo.Object
	for _, t := range types {
		if object, ok := isService(t); ok {
			services = append(services, object)
			object.Parent.Object = &tygo.Object{
				Name:   "Entity",
				Parent: &tygo.InstanceType{PkgName: "tygo", PkgPath: tygo.TYGO_PATH, Name: "Tygo"},
			}
		}
	}
	tygo.Inject(path, filename, pkgname, types)
	injectfile := SRC_PATH + path + "/" + strings.Replace(filename, ".go", ".rpc.go", 1)
	if len(services) == 0 {
		os.Remove(injectfile)
		return
	}

	var head bytes.Buffer
	var body bytes.Buffer
	head.Write([]byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

package %s
`, pkgname)))
	body.Write([]byte(`
`))

	var pkgs map[string]string
	for _, service := range services {
		srv_s, srv_p := injectService(service, service.Name+"Local")
		body.Write([]byte(fmt.Sprintf(`
var %sMutex sync.Mutex
var %sLocal = make(map[ruid.RUID]*%s)
`, service.Name, service.Name, service.Name)))
		body.Write([]byte(srv_s))
		pkgs = update(pkgs, srv_p)
		pkgs = update(pkgs, LOCAL_PKG)
	}

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
	ioutil.WriteFile(injectfile, head.Bytes(), 0666)
}

func injectProcedureCaller(owner string, service string, method *tygo.Method, local string) (string, map[string]string) {
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
		param = fmt.Sprintf("s.Serialize%sParam(%s)", method.Name, strings.Join(params_list, ", "))
	} else {
		param = "nil"
	}

	var result_local string
	var result_remote string
	var results_list []string
	var results_declare []string
	for i, r := range method.Results {
		result_s, result_p := r.Go()
		pkgs = update(pkgs, result_p)
		results_list = append(results_list, fmt.Sprintf("r%d", i))
		results_declare = append(results_declare, fmt.Sprintf("r%d %s", i, result_s))
	}
	results_declare = append(results_declare, "err error")

	if len(results_list) > 0 {
		result_local = fmt.Sprintf(`
		%s = local.%s(%s)`, strings.Join(results_list, ", "), method.Name, strings.Join(params_list, ", "))
		result_remote = fmt.Sprintf(`
	var result string
	if result, err = Server.Procedure(s.RUID, s.Key, "%s", "%s", %s); err != nil {
		return
	}
	%s, err = s.Deserialize%sResult(result)`, service, method.Name, param,
			strings.Join(results_list, ", "), method.Name)
	} else {
		result_local = fmt.Sprintf(`
		local.%s(%s)`, method.Name, strings.Join(params_list, ", "))
		result_remote = fmt.Sprintf(`
	_, err = Server.Procedure(s.RUID, s.Key, "%s", "%s", %s)`, service, method.Name, param)
	}

	var checkLocal string
	if local != "" {
		checkLocal = fmt.Sprintf(`
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("[%s] Procedure %s panic:\n>>>>%%v", e)
		}
	}()
	if local, exist := %s[s.RUID]; exist {%s
		return
	}`, service, method.Name, local, result_local)
		pkgs = update(pkgs, FMT_PKG)
	}

	return fmt.Sprintf(`
func (s *%s) %s(%s) (%s) {%s%s
	return
}
`, owner, method.Name, strings.Join(params_declare, ", "), strings.Join(results_declare, ", "),
		checkLocal, result_remote), pkgs
}

func injectService(service *tygo.Object, local string) (string, map[string]string) {
	var pkgs map[string]string
	var methods []string
	for _, method := range service.Methods {
		param_s, param_p := tygo.TypeListSerialize(service.Name+"Delegate", method.Name, "param", method.Params)
		result_s, result_p := tygo.TypeListDeserialize(service.Name+"Delegate", method.Name, "result", method.Results)
		method_s, method_p := injectProcedureCaller(service.Name+"Delegate", service.Name, method, local)
		pkgs = update(pkgs, param_p)
		pkgs = update(pkgs, result_p)
		pkgs = update(pkgs, method_p)
		methods = append(methods, param_s)
		methods = append(methods, result_s)
		methods = append(methods, method_s)
	}

	return fmt.Sprintf(`
type %sDelegate Entity

func (e *Entity) %s() *%sDelegate {
	return (*%sDelegate)(e)
}
%s`, service.Name, service.Name, service.Name, service.Name, strings.Join(methods, "")), pkgs
}
