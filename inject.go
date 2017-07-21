// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"go/doc"
	"io/ioutil"

	"github.com/ibelie/tygo"
)

var (
	FMT_PKG   = map[string]string{"fmt": ""}
	LOCAL_PKG = map[string]string{
		"fmt":                    "",
		"sync":                   "",
		"github.com/ibelie/rpc":  "",
		"github.com/ibelie/ruid": "",
		"github.com/ibelie/tygo": "",
	}
	PROP_PRE = []tygo.Type{tygo.SimpleType_UINT64, tygo.SimpleType_UINT64}
	DELEGATE = "Delegate"
)

func ReplaceEntity(dir string, filename string, pkgname string, types []tygo.Type) {
	for _, t := range types {
		if object, ok := isService(t); ok {
			object.Parent.Object = &tygo.Object{
				Name:   "Entity",
				Parent: &tygo.InstanceType{PkgName: "tygo", PkgPath: tygo.TYGO_PATH, Name: "Tygo"},
			}
		}
	}
}

func Inject(dir string, filename string, pkgname string, types []tygo.Type) {
	ReplaceEntity(dir, filename, pkgname, types)
	var services []*tygo.Object
	for _, t := range types {
		if object, ok := isService(t); ok {
			services = append(services, object)
		}
	}
	tygo.PROP_PRE = PROP_PRE
	tygo.DELEGATE = DELEGATE
	tygo.Inject(dir, filename, pkgname, types)
	tygo.PROP_PRE = nil
	tygo.DELEGATE = ""
	injectfile := path.Join(SRC_PATH, dir, strings.Replace(filename, ".go", ".rpc.go", 1))
	if len(services) == 0 {
		os.Remove(injectfile)
		return
	}

	objects := make(map[string]*doc.Type)
	if d := packageDoc(dir); d != nil {
		for _, t := range d.Types {
			for _, s := range services {
				if t.Name == s.Name {
					objects[t.Name] = t
				}
			}
		}
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
		var methods []string
		local_m, local_s, local_p := injectServiceLocal(service, objects[service.Name])
		property_s, property_p := injectServiceProperty(service)
		for _, method := range service.Methods {
			method_s, method_p := injectProcedureCaller(service.Name+DELEGATE, service.Name, method, local_m)
			pkgs = update(pkgs, method_p)
			methods = append(methods, method_s)
		}

		body.Write([]byte(local_s))
		body.Write([]byte(property_s))
		body.Write([]byte(fmt.Sprintf(`
type %sDelegate Entity

func (e *Entity) %s() *%sDelegate {
	return (*%sDelegate)(e)
}
%s`, service.Name, service.Name, service.Name, service.Name, strings.Join(methods, ""))))
		pkgs = update(pkgs, local_p)
		pkgs = update(pkgs, property_p)
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
	ioutil.WriteFile(injectfile, head.Bytes(), 0666)
}

func injectProcedureCallee(service *tygo.Object, method *tygo.Method) (string, map[string]string) {
	var param string
	var params_list []string
	for i, _ := range method.Params {
		params_list = append(params_list, fmt.Sprintf("p%d", i))
	}
	if len(params_list) > 0 {
		param = fmt.Sprintf(` else if %s, e := service.Deserialize%sParam(param); e != nil {
			err = e
		}`,
			strings.Join(params_list, ", "), method.Name)
	}

	var result string
	if len(method.Results) > 0 {
		result = fmt.Sprintf(` else {
			result = service.Serialize%sResult(service.%s(%s))
		}`,
			method.Name, method.Name, strings.Join(params_list, ", "))
	} else {
		result = fmt.Sprintf(` else {
			service.%s(%s)
		}`, method.Name, strings.Join(params_list, ", "))
	}

	return fmt.Sprintf(`
	case SYMBOL_%s:
		methodName = "%s"
		if service, exist := s.services[i]; !exist {
			err = fmt.Errorf("[%s] Service %s RUID not exists: %%v", i)
		}%s%s
`, method.Name, method.Name, service.Name, method.Name, param, result), FMT_PKG
}

func injectServiceLocal(service *tygo.Object, object *doc.Type) (string, string, map[string]string) {
	var onCreate string
	if hasMethod(object, &tygo.Method{Name: "onCreate"}) {
		onCreate = `
			s.services[i].onCreate()`
	}

	serviceTemp := "_"
	var onDestroy string
	if hasMethod(object, &tygo.Method{Name: "onDestroy"}) {
		serviceTemp = "service"
		onDestroy = `
			service.onDestroy()`
	}

	pkgs := LOCAL_PKG
	var cases []string
	for _, m := range service.Methods {
		if !hasMethod(object, m) {
			continue
		}
		case_s, case_p := injectProcedureCallee(service, m)
		cases = append(cases, case_s)
		pkgs = update(pkgs, case_p)
	}

	return fmt.Sprintf("%sInst.services", service.Name), fmt.Sprintf(`
type %sServiceImpl struct {
	mutex    sync.Mutex
	services map[ruid.RUID]*%s
}

var %sInst = %sServiceImpl{services: make(map[ruid.RUID]*%s)}

func %sService(server rpc.IServer, symbols map[string]uint64) (uint64, rpc.Service) {
	InitializeServer(server, symbols)
	return SYMBOL_%s, &%sInst
}

func (s *%sServiceImpl) Procedure(i ruid.RUID, method uint64, param []byte) (result []byte, err error) {
	var methodName string
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("[%s] Procedure %%s(%%d) %%v panic:\n>>>> %%v", methodName, method, i, e)
		}
	}()

	switch method {
	case SYMBOL_CREATE:
		methodName = "Create"
		s.mutex.Lock()
		defer s.mutex.Unlock()
		if _, exist := s.services[i]; exist {
			err = fmt.Errorf("[%s] Service create RUID already exists: %%v", i)
		} else {
			input := &tygo.ProtoBuf{Buffer: param}
			var k, t uint64
			if k, err = input.ReadVarint(); err != nil {
				return
			} else if t, err = input.ReadVarint(); err != nil {
				return
			}
			s.services[i] = &%s{Entity: &Entity{RUID: i, Key: ruid.RUID(k), Type: t}}%s
		}
	case SYMBOL_DESTROY:
		methodName = "Destroy"
		s.mutex.Lock()
		defer s.mutex.Unlock()
		if %s, exist := s.services[i]; !exist {
			err = fmt.Errorf("[%s] Service destroy RUID not exists: %%v", i)
		} else {%s
			delete(s.services, i)
		}
	case SYMBOL_SYNCHRON:
		methodName = "Synchron"
		if service, exist := s.services[i]; !exist {
			err = fmt.Errorf("[%s] Service synchron RUID not exists: %%v", i)
		} else {
			result = make([]byte, tygo.SizeVarint(SYMBOL_%s) + tygo.SizeVarint(uint64(service.ByteSize())) + service.ByteSize())
			protobuf := &tygo.ProtoBuf{Buffer: result}
			protobuf.WriteVarint(SYMBOL_%s)
			protobuf.WriteVarint(uint64(service.ByteSize()))
			service.Serialize(protobuf)
		}%s
	}
	return
}
`, service.Name, service.Name, service.Name, service.Name, service.Name, service.Name,
		service.Name, service.Name, service.Name, service.Name, service.Name, service.Name,
		onCreate, serviceTemp, service.Name, onDestroy, service.Name, service.Name,
		service.Name, strings.Join(cases, "")), pkgs
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
	var result []byte
	if result, err = Server.Procedure(s.RUID, s.Key, SYMBOL_%s, SYMBOL_%s, %s); err != nil {
		return
	}
	%s, err = s.Deserialize%sResult(result)`, service, method.Name, param,
			strings.Join(results_list, ", "), method.Name)
	} else {
		result_local = fmt.Sprintf(`
		local.%s(%s)`, method.Name, strings.Join(params_list, ", "))
		result_remote = fmt.Sprintf(`
	_, err = Server.Procedure(s.RUID, s.Key, SYMBOL_%s, SYMBOL_%s, %s)`,
			service, method.Name, param)
	}

	var checkLocal string
	if local != "" {
		checkLocal = fmt.Sprintf(`
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("[%s] Procedure %s panic:\n>>>> %%v", e)
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

func injectServiceProperty(service *tygo.Object) (string, map[string]string) {
	var pkgs map[string]string
	var codes []string
	for _, property := range service.Fields {
		switch t := property.Type.(type) {
		case *tygo.ListType:
			element_s, element_p := t.E.Go()
			pkgs = update(pkgs, element_p)
			codes = append(codes, fmt.Sprintf(`
func (s *%s) append%s(x ...%s) error {
	if len(x) <= 0 {
		return nil
	}
	s.%s = append(s.%s, x...)
	e := s.Entity
	return Server.Notify(e.RUID, e.Key, s.Serialize%s(SYMBOL_%s, SYMBOL_%s, x))
}
`, service.Name, strings.Title(property.Name), element_s, property.Name, property.Name,
				property.Name, service.Name, property.Name))
		case *tygo.DictType:
			property_s, property_p := property.Go()
			pkgs = update(pkgs, property_p)
			codes = append(codes, fmt.Sprintf(`
func (s *%s) update%s(x %s) error {
	if x == nil || len(x) <= 0 {
		return nil
	} else if s.%s == nil {
		s.%s = make(%s)
	}
	for k, v := range x {
		s.%s[k] = v
	}
	e := s.Entity
	return Server.Notify(e.RUID, e.Key, s.Serialize%s(SYMBOL_%s, SYMBOL_%s, x))
}
`, service.Name, strings.Title(property.Name), property_s, property.Name, property.Name,
				property_s, property.Name, property.Name, service.Name, property.Name))
		default:
			property_s, property_p := property.Go()
			pkgs = update(pkgs, property_p)
			codes = append(codes, fmt.Sprintf(`
func (s *%s) set%s(x %s) error {
	s.%s = x
	e := s.Entity
	return Server.Notify(e.RUID, e.Key, s.Serialize%s(SYMBOL_%s, SYMBOL_%s, x))
}
`, service.Name, strings.Title(property.Name), property_s, property.Name, property.Name,
				service.Name, property.Name))
		}

	}

	return strings.Join(codes, ""), pkgs
}
