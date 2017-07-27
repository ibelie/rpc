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

var (
	SYMBOL_SESSION  uint64
	OBSERVE_SESSION []byte
	HANDSHAKE_DATA  []byte
)

type GateImpl struct {
	mutex sync.Mutex
	gates map[ruid.ID]Connection
}

var GateInst = GateImpl{gates: make(map[ruid.ID]Connection)}

func GateService(_ Server, _ map[string]uint64) (uint64, Service) {
	return SYMBOL_GATE, &GateInst
}

func Gate(address string, session string, network Network) {
	SYMBOL_SESSION = server.symbols[session]
	OBSERVE_SESSION = make([]byte, tygo.SizeVarint(SYMBOL_SESSION<<1))
	(&tygo.ProtoBuf{Buffer: OBSERVE_SESSION}).WriteVarint(SYMBOL_SESSION << 1)

	syms := 0
	for name, value := range server.symbols {
		symbol := []byte(name)
		syms += tygo.SizeVarint(uint64(len(symbol))) + len(symbol) + tygo.SizeVarint(value)
	}
	HANDSHAKE_DATA = make([]byte, syms+tygo.SizeVarint(uint64(syms))+tygo.SizeVarint(SYMBOL_SESSION))
	output := &tygo.ProtoBuf{Buffer: HANDSHAKE_DATA}
	output.WriteVarint(uint64(syms))
	for symbol, value := range server.symbols {
		output.WriteBuf([]byte(symbol))
		output.WriteVarint(value)
	}
	output.WriteVarint(SYMBOL_SESSION)

	network.Serve(address, GateInst.handler)
}

func (s *GateImpl) observe(i ruid.ID, k ruid.ID, t uint64, session ruid.ID) (components [][]byte, err error) {
	if components, err = server.Distribute(i, k, t, SYMBOL_SYNCHRON, nil); err != nil {
		err = fmt.Errorf("[Gate] Synchron %s(%v:%v) error %v:\n>>>> %v", server.symdict[t], i, k, session, err)
		return
	}
	_, err = server.Procedure(i, k, SYMBOL_HUB, SYMBOL_OBSERVE, SerializeSessionGate(session, server.Addr))
	return
}

func (s *GateImpl) ignore(i ruid.ID, k ruid.ID, session ruid.ID) (err error) {
	_, err = server.Procedure(i, k, SYMBOL_HUB, SYMBOL_IGNORE, SerializeSessionGate(session, server.Addr))
	return
}

func (s *GateImpl) handler(gate Connection) {
	session := server.Ident.New()
	if _, err := server.Distribute(session, server.ServerID(), SYMBOL_SESSION, SYMBOL_CREATE, OBSERVE_SESSION); err != nil {
		log.Printf("[Gate@%v] Create session error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
	} else if err := gate.Send(SerializeHandshake(session, components)); err != nil {
		log.Printf("[Gate@%v] Send session error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
	}

	GateInst.mutex.Lock()
	s.gates[session] = gate
	GateInst.mutex.Unlock()

	for {
		data, err := gate.Receive()
		if err != nil {
			log.Printf("[Gate@%v] Receive error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
			break
		}
		var t, m uint64
		var i, k ruid.ID
		input := &tygo.ProtoBuf{Buffer: data}
		if t, err = input.ReadVarint(); err != nil {
			log.Printf("[Gate@%v] Read Type error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
			break
		}
		if t&1 == 0 {
			i = server.ZeroID()
		} else if i, err = server.DeserializeID(input); err != nil {
			log.Printf("[Gate@%v] Read ID error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
			break
		}
		if t&2 == 0 {
			k = server.ZeroID()
		} else if k, err = server.DeserializeID(input); err != nil {
			log.Printf("[Gate@%v] Read Key error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
			break
		}
		if m, err = input.ReadVarint(); err != nil {
			log.Printf("[Gate@%v] Read method error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
			break
		}
		t >>= 2
		switch m {
		case SYMBOL_OBSERVE:
			if components, err := s.observe(i, k, t, session); err != nil {
				log.Printf("[Gate@%v] Observe %s(%v:%v) error %v %v:\n>>>> %v", server.Addr, server.symdict[t], i, k, gate.Address(), session, err)
			} else if err := gate.Send(SerializeSynchron(i, components)); err != nil {
				log.Printf("[Gate@%v] Send %s(%v:%v) error %v %v:\n>>>> %v", server.Addr, server.symdict[t], i, k, gate.Address(), session, err)
			}
		case SYMBOL_IGNORE:
			if err := s.ignore(i, k, session); err != nil {
				log.Printf("[Gate@%v] Ignore %s(%v:%v) error %v %v:\n>>>> %v", server.Addr, server.symdict[t], i, k, gate.Address(), session, err)
			}
		default:
			if _, err := server.Distribute(i, k, t, m, input.Bytes()); err != nil {
				log.Printf("[Gate@%v] Distribute %s(%v) to %s(%v:%v) error %v %v:\n>>>> %v", server.Addr, server.symdict[m], m, server.symdict[t], i, k, gate.Address(), session, err)
			}
		}
	}

	GateInst.mutex.Lock()
	delete(s.gates, session)
	GateInst.mutex.Unlock()
	if _, err := server.Distribute(session, server.ServerID(), SYMBOL_SESSION, SYMBOL_DESTROY, nil); err != nil {
		log.Printf("[Gate@%v] Destroy session error %v:\n>>>> %v", server.Addr, session, err)
	}
}

func (s *GateImpl) Procedure(i ruid.ID, method uint64, param []byte) (result []byte, err error) {
	var observers []ruid.ID
	if observers, param, err = DeserializeDispatch(param); err != nil {
		err = fmt.Errorf("[Gate] Dispatch deserialize error: %v %s(%v)\n>>>> %v", i, server.symdict[method], method, err)
		return
	}
	size := i.ByteSize() + tygo.SizeVarint(method) + tygo.SizeVarint(uint64(len(param))) + len(param)
	data := make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	i.Serialize(output)
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
	if len(errors) > 0 {
		err = fmt.Errorf("[Gate] Dispatch errors:\n>>>> %v", strings.Join(errors, ""))
	}
	return
}

func SerializeSessionGate(session ruid.ID, gate string) (data []byte) {
	g := []byte(gate)
	data = make([]byte, len(g)+session.ByteSize())
	output := &tygo.ProtoBuf{Buffer: data}
	session.Serialize(output)
	output.Write(g)
	return
}

func DeserializeSessionGate(data []byte) (session ruid.ID, gate string, err error) {
	input := &tygo.ProtoBuf{Buffer: data}
	if session, err = server.DeserializeID(input); err == nil {
		gate = string(input.Bytes())
	}
	return
}

func SerializeDispatch(observers map[ruid.ID]bool, param []byte) (data []byte) {
	var size int
	for observer, ok := range observers {
		if ok {
			size += observer.ByteSize()
		}
	}
	data = make([]byte, tygo.SizeVarint(uint64(size))+size+len(param))
	output := &tygo.ProtoBuf{Buffer: data}
	output.WriteVarint(uint64(size))
	for observer, ok := range observers {
		if ok {
			observer.Serialize(output)
		}
	}
	output.Write(param)
	return
}

func DeserializeDispatch(data []byte) (observers []ruid.ID, param []byte, err error) {
	var o []byte
	var observer ruid.ID
	input := &tygo.ProtoBuf{Buffer: data}
	if o, err = input.ReadBuf(); err == nil {
		buffer := &tygo.ProtoBuf{Buffer: o}
		param = input.Bytes()
		for !buffer.ExpectEnd() {
			if observer, err = server.DeserializeID(buffer); err != nil {
				return
			} else {
				observers = append(observers, observer)
			}
		}
	}
	return
}

func SerializeSynchron(i ruid.ID, components [][]byte) (data []byte) {
	size := i.ByteSize()
	for _, component := range components {
		size += len(component)
	}

	data = make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	i.Serialize(output)
	for _, component := range components {
		output.Write(component)
	}
	return
}

func SerializeHandshake(i ruid.ID, components [][]byte) (data []byte) {
	size := len(HANDSHAKE_DATA) + i.ByteSize()
	for _, component := range components {
		size += len(component)
	}

	data = make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	i.Serialize(output)
	output.Write(HANDSHAKE_DATA)
	for _, component := range components {
		output.Write(component)
	}
	return
}
