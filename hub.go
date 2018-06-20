// Copyright 2017-2018 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"log"
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
			log.Printf("[Hub@%v] Dispatch %q no observer for %v", server.Addr, m, i)
			return
		}
		var errors []string
		gateGhosts := make(map[string][]ruid.ID)
		for gate, observers := range gates {
			if len(observers) <= 0 {
				gateGhosts[gate] = nil
				continue
			}
			var datas [][]byte
			var ghosts []ruid.ID
			if datas, err = server.Request(gate, i, m, SerializeDispatch(observers, param), SYMBOL_GATE); err != nil || len(datas) <= 0 {
				errors = append(errors, fmt.Sprintf("\n>>>> gate request: %v\n>>>> %v", gate, err))
			} else if ghosts, err = DeserializeGhosts(datas[0]); err != nil {
				errors = append(errors, fmt.Sprintf("\n>>>> gate ghosts: %v\n>>>> %v", gate, err))
			} else if len(ghosts) > 0 {
				gateGhosts[gate] = ghosts
			}
		}
		for gate, ghosts := range gateGhosts {
			for _, ghost := range ghosts {
				delete(gates[gate], ghost)
			}
			if len(gates[gate]) <= 0 {
				delete(gates, gate)
			}
		}
		if len(errors) > 0 {
			err = fmt.Errorf("[Hub] Dispatch %q errors %v:%s", m, i, strings.Join(errors, ""))
		}
	}
	return
}

func DeserializeSessionGate(data []byte) (session ruid.ID, gate string, err error) {
	var gateID ruid.ID
	input := &tygo.ProtoBuf{Buffer: data}
	if session, err = server.DeserializeID(input); err != nil {
	} else if gateID, err = server.DeserializeID(input); err != nil {
	} else if ring, ok := server.remote[SYMBOL_GATE]; !ok {
		err = fmt.Errorf("Cannot find gate service: %v %v %v", server.remote, server.Node, server.nodes)
	} else if gate, ok = ring.Get(gateID); !ok {
		err = fmt.Errorf("No gate found: %v %v", server.Node, server.nodes)
	}
	return
}
