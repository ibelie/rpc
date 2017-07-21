// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
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

type Gate interface {
	Address() string
	Send([]byte) error
	Receive() ([]byte, error)
	Close() error
}

type GateImpl struct {
	mutex sync.Mutex
	gates map[ruid.RUID]Gate
}

var GateInst = GateImpl{gates: make(map[ruid.RUID]Gate)}

func GateService(server IServer, symbols map[string]uint64) (uint64, Service) {
	return SYMBOL_GATE, &GateInst
}

func (s *GateImpl) observe(i ruid.RUID, k ruid.RUID, t uint64, session ruid.RUID) (components [][]byte, err error) {
	comChan := make(chan []byte)
	if err = ServerInst.Distribute(i, k, t, SYMBOL_SYNCHRON, nil, comChan); err != nil {
		err = fmt.Errorf("[Gate] Synchron %s(%v:%v) error %v:\n>>>> %v", ServerInst.symdict[t], i, k, session, err)
		return
	}
	for component := range comChan {
		components = append(components, component)
	}
	_, err = ServerInst.Procedure(i, k, SYMBOL_HUB, SYMBOL_OBSERVE, SerializeSessionGate(session, ServerInst.Address))
	return
}

func (s *GateImpl) ignore(i ruid.RUID, k ruid.RUID, session ruid.RUID) (err error) {
	_, err = ServerInst.Procedure(i, k, SYMBOL_HUB, SYMBOL_IGNORE, SerializeSessionGate(session, ServerInst.Address))
	return
}

func (s *GateImpl) handler(gate Gate) {
	session := ruid.New()
	if err := ServerInst.Distribute(session, 0, SYMBOL_SESSION, SYMBOL_CREATE, []byte{0, byte(SYMBOL_SESSION)}, nil); err != nil {
		log.Printf("[Gate@%v] Create session error %v %v:\n>>>> %v", ServerInst.Address, gate.Address(), session, err)
	} else if components, err := s.observe(session, 0, SYMBOL_SESSION, session); err != nil {
		log.Printf("[Gate@%v] Observe session error %v %v:\n>>>> %v", ServerInst.Address, gate.Address(), session, err)
	} else if err := gate.Send(SerializeHandshake(session, ServerInst.symbols, components)); err != nil {
		log.Printf("[Gate@%v] Send session error %v %v:\n>>>> %v", ServerInst.Address, gate.Address(), session, err)
	}

	GateInst.mutex.Lock()
	s.gates[session] = gate
	GateInst.mutex.Unlock()

	for {
		data, err := gate.Receive()
		if err != nil {
			log.Printf("[Gate@%v] Receive error %v %v:\n>>>> %v", ServerInst.Address, gate.Address(), session, err)
			break
		}
		input := &tygo.ProtoBuf{Buffer: data}
		if i, err := input.ReadVarint(); err != nil {
			log.Printf("[Gate@%v] Read RUID error %v %v:\n>>>> %v", ServerInst.Address, gate.Address(), session, err)
			break
		} else if k, err := input.ReadVarint(); err != nil {
			log.Printf("[Gate@%v] Read Key error %v %v:\n>>>> %v", ServerInst.Address, gate.Address(), session, err)
			break
		} else if t, err := input.ReadVarint(); err != nil {
			log.Printf("[Gate@%v] Read Type error %v %v:\n>>>> %v", ServerInst.Address, gate.Address(), session, err)
			break
		} else if m, err := input.ReadVarint(); err != nil {
			log.Printf("[Gate@%v] Read method error %v %v:\n>>>> %v", ServerInst.Address, gate.Address(), session, err)
			break
		} else {
			switch m {
			case SYMBOL_OBSERVE:
				if components, err := s.observe(ruid.RUID(i), ruid.RUID(k), t, session); err != nil {
					log.Printf("[Gate@%v] Observe %s(%v:%v) error %v %v:\n>>>> %v", ServerInst.Address, ServerInst.symdict[t], ruid.RUID(i), ruid.RUID(k), gate.Address(), session, err)
				} else if err := gate.Send(SerializeSynchron(ruid.RUID(i), components)); err != nil {
					log.Printf("[Gate@%v] Send %s(%v:%v) error %v %v:\n>>>> %v", ServerInst.Address, ServerInst.symdict[t], ruid.RUID(i), ruid.RUID(k), gate.Address(), session, err)
				}
			case SYMBOL_IGNORE:
				if err := s.ignore(ruid.RUID(i), ruid.RUID(k), session); err != nil {
					log.Printf("[Gate@%v] Ignore %s(%v:%v) error %v %v:\n>>>> %v", ServerInst.Address, ServerInst.symdict[t], ruid.RUID(i), ruid.RUID(k), gate.Address(), session, err)
				}
			default:
				if err := ServerInst.Distribute(ruid.RUID(i), ruid.RUID(k), t, m, input.Bytes(), nil); err != nil {
					log.Printf("[Gate@%v] Distribute %s(%v) to %s(%v:%v) error %v %v:\n>>>> %v", ServerInst.Address, ServerInst.symdict[m], m, ServerInst.symdict[t], ruid.RUID(i), ruid.RUID(k), gate.Address(), session, err)
				}
			}
		}
	}

	GateInst.mutex.Lock()
	delete(s.gates, session)
	GateInst.mutex.Unlock()
	if err := s.ignore(session, 0, session); err != nil {
		log.Printf("[Gate@%v] Ignore session error %v:\n>>>> %v", ServerInst.Address, session, err)
	} else if err := ServerInst.Distribute(session, 0, SYMBOL_SESSION, SYMBOL_DESTROY, nil, nil); err != nil {
		log.Printf("[Gate@%v] Destroy session error %v:\n>>>> %v", ServerInst.Address, session, err)
	}
}

func (s *GateImpl) Procedure(i ruid.RUID, method uint64, param []byte) (result []byte, err error) {
	var observers []ruid.RUID
	if observers, param, err = DeserializeDispatch(param); err != nil {
		err = fmt.Errorf("[Gate] Dispatch deserialize error: %v %s(%v)\n>>>> %v", i, ServerInst.symdict[method], method, err)
		return
	}
	size := tygo.SizeVarint(uint64(i)) + tygo.SizeVarint(method) +
		tygo.SizeVarint(uint64(len(param))) + len(param)
	data := make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	output.WriteVarint(uint64(i))
	output.WriteVarint(method)
	output.WriteVarint(uint64(len(param)))
	output.Write(param)
	var errors []string
	for _, observer := range observers {
		if gate, ok := s.gates[observer]; !ok {
			errors = append(errors, fmt.Sprintf("\n>>>> Dispatch gate not found %v", observer))
		} else if err = gate.Send(data); err != nil {
			errors = append(errors, fmt.Sprintf("\n>>>> gate: %v\n>>>> %v", gate, err))
			break
		}
	}
	err = fmt.Errorf("[Gate] Dispatch errors:\n>>>> %v", strings.Join(errors, ""))
	return
}

func SerializeSessionGate(session ruid.RUID, gate string) (data []byte) {
	g := []byte(gate)
	data = make([]byte, tygo.SizeVarint(uint64(session))+len(g))
	output := &tygo.ProtoBuf{Buffer: data}
	output.WriteVarint(uint64(session))
	output.Write(g)
	return
}

func DeserializeSessionGate(data []byte) (session ruid.RUID, gate string, err error) {
	input := &tygo.ProtoBuf{Buffer: data}
	if s, e := input.ReadVarint(); e != nil {
		err = e
	} else {
		session = ruid.RUID(s)
		gate = string(input.Bytes())
	}
	return
}

func SerializeDispatch(observers map[ruid.RUID]bool, param []byte) (data []byte) {
	var size int
	for observer, ok := range observers {
		if ok {
			size += tygo.SizeVarint(uint64(observer))
		}
	}
	data = make([]byte, tygo.SizeVarint(uint64(size))+size+len(param))
	output := &tygo.ProtoBuf{Buffer: data}
	output.WriteVarint(uint64(size))
	for observer, ok := range observers {
		if ok {
			output.WriteVarint(uint64(observer))
		}
	}
	output.Write(param)
	return
}

func DeserializeDispatch(data []byte) (observers []ruid.RUID, param []byte, err error) {
	var o []byte
	var observer uint64
	input := &tygo.ProtoBuf{Buffer: data}
	if o, err = input.ReadBuf(); err == nil {
		buffer := &tygo.ProtoBuf{Buffer: o}
		param = input.Bytes()
		for !buffer.ExpectEnd() {
			if observer, err = buffer.ReadVarint(); err != nil {
				return
			} else {
				observers = append(observers, ruid.RUID(observer))
			}
		}
	}
	return
}

func SerializeSynchron(i ruid.RUID, components [][]byte) (data []byte) {
	size := tygo.SizeVarint(uint64(i))
	for _, component := range components {
		size += len(component)
	}

	data = make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	output.WriteVarint(uint64(i))
	for _, component := range components {
		output.Write(component)
	}
	return
}

func SerializeHandshake(i ruid.RUID, symbols map[string]uint64, components [][]byte) (data []byte) {
	symbolsSize := 0
	for name, value := range symbols {
		symbol := []byte(name)
		symbolsSize += tygo.SizeVarint(uint64(len(symbol))) + len(symbol) + tygo.SizeVarint(value)
	}
	size := symbolsSize + tygo.SizeVarint(uint64(symbolsSize)) + tygo.SizeVarint(uint64(i))
	for _, component := range components {
		size += len(component)
	}

	data = make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	output.WriteVarint(uint64(i))
	output.WriteVarint(uint64(symbolsSize))
	for symbol, value := range symbols {
		output.WriteBuf([]byte(symbol))
		output.WriteVarint(value)
	}
	for _, component := range components {
		output.Write(component)
	}
	return
}
