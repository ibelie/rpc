// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ibelie/ruid"
	"github.com/ibelie/tygo"
)

type HubImpl struct {
	mutex     sync.Mutex
	observers map[ruid.ID]map[string]map[ruid.ID]bool
}

var HubInst = HubImpl{observers: make(map[ruid.ID]map[string]map[ruid.ID]bool)}

func HubService(_ Server) (string, Service) {
	return SYMBOL_HUB, &HubInst
}

func (s *HubImpl) Procedure(i ruid.ID, m string, param []byte) (result []byte, err error) {
	switch m {
	case SYMBOL_OBSERVE:
		if session, gate, e := DeserializeSessionGate(param); e != nil {
			err = fmt.Errorf("[Hub] Observe deserialize error: %v\n>>>> %v", i, e)
		} else {
			s.mutex.Lock()
			defer s.mutex.Unlock()
			if _, ok := s.observers[i]; !ok {
				s.observers[i] = make(map[string]map[ruid.ID]bool)
			}
			if _, ok := s.observers[i][gate]; !ok {
				s.observers[i][gate] = make(map[ruid.ID]bool)
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
			if _, err = server.Request(gate, i, m, SerializeDispatch(observers, param), SYMBOL_GATE); err != nil {
				errors = append(errors, fmt.Sprintf("\n>>>> gate: %v\n>>>> %v", gate, err))
			}
		}
		if len(errors) > 0 {
			err = fmt.Errorf("[Hub] Dispatch %q errors %v:%s", m, i, strings.Join(errors, ""))
		}
	}
	return
}

func DeserializeSessionGate(data []byte) (session ruid.ID, gate string, err error) {
	if session, err = server.DeserializeID(&tygo.ProtoBuf{Buffer: data}); err != nil {
	} else if ring, ok := server.remote[SYMBOL_GATE]; !ok {
		err = fmt.Errorf("Cannot find gate service: %v %v %v", server.remote, server.Node, server.nodes)
	} else if gate, ok = ring.Get(session); !ok {
		err = fmt.Errorf("No gate found: %v %v", server.Node, server.nodes)
	}
	return
}
