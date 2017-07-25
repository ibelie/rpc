// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"strings"
	"sync"

	id "github.com/ibelie/ruid"
)

type HubImpl struct {
	mutex     sync.Mutex
	observers map[id.ID]map[string]map[id.ID]bool
}

var HubInst = HubImpl{observers: make(map[id.ID]map[string]map[id.ID]bool)}

func HubService(_ Server, _ map[string]uint64) (uint64, Service) {
	return SYMBOL_HUB, &HubInst
}

func (s *HubImpl) Procedure(i id.ID, method uint64, param []byte) (result []byte, err error) {
	switch method {
	case SYMBOL_OBSERVE:
		if session, gate, e := DeserializeSessionGate(param); e != nil {
			err = fmt.Errorf("[Hub] Observe deserialize error: %v\n>>>> %v", i, e)
		} else {
			s.mutex.Lock()
			defer s.mutex.Unlock()
			if _, ok := s.observers[i]; !ok {
				s.observers[i] = make(map[string]map[id.ID]bool)
			}
			if _, ok := s.observers[i][gate]; !ok {
				s.observers[i][gate] = make(map[id.ID]bool)
			}
			s.observers[i][gate][session] = true
		}
	case SYMBOL_IGNORE:
		if session, gate, e := DeserializeSessionGate(param); e != nil {
			err = fmt.Errorf("[Hub] Ignore deserialize error: %v\n>>>> %v", i, e)
		} else {
			s.mutex.Lock()
			defer s.mutex.Unlock()
			if gates, ok := s.observers[i]; ok {
				if observers, ok := gates[gate]; ok {
					delete(observers, session)
					if len(observers) <= 0 {
						delete(gates, gate)
					}
				}
				if len(gates) <= 0 {
					delete(s.observers, i)
				}
			}
		}
	default:
		gates, ok := s.observers[i]
		if !ok || len(gates) <= 0 {
			err = fmt.Errorf("[Hub] Dispatch no gate found %v %v", i, gates)
			return
		}
		var errors []string
		for gate, observers := range gates {
			if len(observers) <= 0 {
				errors = append(errors, fmt.Sprintf("\n>>>> [Hub] Dispatch gate no observer %v %v", i, gate))
				continue
			}
			if _, err = server.Request(gate, i, method, SerializeDispatch(observers, param), SYMBOL_GATE); err != nil {
				errors = append(errors, fmt.Sprintf("\n>>>> gate: %v\n>>>> %v", gate, err))
			}
		}
		if len(errors) > 0 {
			err = fmt.Errorf("[Hub] Dispatch errors %v %s(%v):%s", i, server.symdict[method], method, strings.Join(errors, ""))
		}
	}
	return
}
