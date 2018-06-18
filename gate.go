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
	SYMBOL_SESSION string
	CREATE_SESSION []byte
	HANDSHAKE_DATA []byte
	GATE_SYMBOLS   []string
	GATE_SYMDICT   = make(map[string]uint64)
)

type GateImpl struct {
	mutex sync.Mutex
	gates map[ruid.ID]Connection
}

var GateInst = GateImpl{gates: make(map[ruid.ID]Connection)}

func GateService(_ Server) (string, Service) {
	return SYMBOL_GATE, &GateInst
}

func Gate(address string, session string, version [16]byte, symbols []string, network Network) {
	SYMBOL_SESSION = session
	serverID := server.ServerID()
	CREATE_SESSION = make([]byte, 1+tygo.SymbolEncodedLen(SYMBOL_SESSION)+serverID.ByteSize())
	output := &tygo.ProtoBuf{Buffer: CREATE_SESSION}
	output.WriteBytes(1)
	serverID.Serialize(output)
	output.EncodeSymbol(SYMBOL_SESSION)

	GATE_SYMBOLS = symbols
	for i, symbol := range GATE_SYMBOLS {
		GATE_SYMDICT[symbol] = uint64(i)
	}
	sessionType := GATE_SYMDICT[SYMBOL_SESSION]
	HANDSHAKE_DATA = make([]byte, len(version)+serverID.ByteSize()+tygo.SizeVarint(sessionType))
	output = &tygo.ProtoBuf{Buffer: HANDSHAKE_DATA}
	output.Write(version[:])
	serverID.Serialize(output)
	output.WriteVarint(sessionType)

	network.Serve(address, GateInst.handler)
}

func (s *GateImpl) handler(gate Connection) {
	session := server.Ident.New()
	serverID := server.ServerID()
	SESSION_BYTES := make([]byte, session.ByteSize()+serverID.ByteSize())
	sessionOutput := &tygo.ProtoBuf{Buffer: SESSION_BYTES}
	session.Serialize(sessionOutput)
	serverID.Serialize(sessionOutput)

	if _, err := server.Distribute(session, server.ServerID(), SYMBOL_SESSION, SYMBOL_CREATE, CREATE_SESSION); err != nil {
		log.Printf("[Gate@%v] Create session error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
	} else if components, err := server.Distribute(session, server.ServerID(), SYMBOL_SESSION, SYMBOL_SYNCHRON, nil); err != nil {
		log.Printf("[Gate@%v] Synchron session error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
	} else if handshake, err := SerializeSynchron(session, components, HANDSHAKE_DATA); err != nil {
		log.Printf("[Gate@%v] Serialize session error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
	} else if err := gate.Send(handshake); err != nil {
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
		var tag, method uint64
		var t, m string
		var i, k ruid.ID
		input := &tygo.ProtoBuf{Buffer: data}
		if tag, err = input.ReadVarint(); err != nil {
			log.Printf("[Gate@%v] Read Type error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
			break
		} else {
			t = GATE_SYMBOLS[tag>>2]
		}
		if tag&1 == 0 {
			i = server.ZeroID()
		} else if i, err = server.DeserializeID(input); err != nil {
			log.Printf("[Gate@%v] Read ID error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
			break
		}
		if tag&2 == 0 {
			k = server.ZeroID()
		} else if k, err = server.DeserializeID(input); err != nil {
			log.Printf("[Gate@%v] Read Key error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
			break
		}
		if method, err = input.ReadVarint(); err != nil {
			log.Printf("[Gate@%v] Read method error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
			break
		} else {
			m = GATE_SYMBOLS[method]
		}
		switch m {
		case SYMBOL_OBSERVE:
			if components, err := server.Distribute(i, k, t, SYMBOL_SYNCHRON, nil); err != nil {
				log.Printf("[Gate@%v] Synchron %s(%v:%v) error %v %v:\n>>>> %v", server.Addr, t, i, k, gate.Address(), session, err)
			} else if _, err = server.Procedure(i, k, t, SYMBOL_HUB, SYMBOL_OBSERVE, SESSION_BYTES); err != nil {
				log.Printf("[Gate@%v] Observe %s(%v:%v) error %v %v:\n>>>> %v", server.Addr, t, i, k, gate.Address(), session, err)
			} else if synchron, err := SerializeSynchron(i, components, nil); err != nil {
				log.Printf("[Gate@%v] Serialize %s(%v:%v) error %v %v:\n>>>> %v", server.Addr, t, i, k, gate.Address(), session, err)
			} else if err := gate.Send(synchron); err != nil {
				log.Printf("[Gate@%v] Send %s(%v:%v) error %v %v:\n>>>> %v", server.Addr, t, i, k, gate.Address(), session, err)
			}
		case SYMBOL_IGNORE:
			if _, err := server.Procedure(i, k, t, SYMBOL_HUB, SYMBOL_IGNORE, SESSION_BYTES); err != nil {
				log.Printf("[Gate@%v] Ignore %s(%v:%v) error %v %v:\n>>>> %v", server.Addr, t, i, k, gate.Address(), session, err)
			}
		default:
			if _, err := server.Distribute(i, k, t, m, input.Bytes()); err != nil {
				log.Printf("[Gate@%v] Distribute %q to %s(%v:%v) error %v %v:\n>>>> %v", server.Addr, m, t, i, k, gate.Address(), session, err)
			}
		}
	}

	GateInst.mutex.Lock()
	delete(s.gates, session)
	GateInst.mutex.Unlock()
	if _, err := server.Distribute(session, server.ServerID(), SYMBOL_SESSION, SYMBOL_DESTROY, nil); err != nil {
		log.Printf("[Gate@%v] Destroy session error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
	}
}

func (s *GateImpl) Procedure(i ruid.ID, m string, param []byte) (result []byte, err error) {
	var observers []ruid.ID
	if observers, param, err = DeserializeDispatch(param); err != nil {
		err = fmt.Errorf("[Gate] Dispatch %q deserialize error: %v\n>>>> %v", m, i, err)
		return
	} else if param, err = ReserializeNotify(m, param); err != nil {
		err = fmt.Errorf("[Gate] Reserialize %q error: %v\n>>>> %v", m, i, err)
		return
	}
	method := GATE_SYMDICT[m]
	size := i.ByteSize() + tygo.SizeVarint(method) + tygo.SizeBuffer(param)
	data := make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	i.Serialize(output)
	output.WriteVarint(method)
	output.WriteVarint(uint64(len(param)))
	output.Write(param)
	var errors []string
	var ghosts []ruid.ID
	for _, observer := range observers {
		if gate, ok := s.gates[observer]; !ok {
			ghosts = append(ghosts, observer)
		} else if err = gate.Send(data); err != nil {
			errors = append(errors, fmt.Sprintf("\n>>>> gate: %v\n>>>> %v", gate, err))
			break
		}
	}

	if len(errors) > 0 {
		err = fmt.Errorf("[Gate] Dispatch errors:\n>>>> %v", strings.Join(errors, ""))
	}
	result = SerializeGhosts(ghosts)
	return
}

func SerializeGhosts(ghosts []ruid.ID) (data []byte) {
	if len(ghosts) <= 0 {
		return
	}
	var size int
	for _, ghost := range ghosts {
		size += ghost.ByteSize()
	}
	data = make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	for _, ghost := range ghosts {
		ghost.Serialize(output)
	}
	return
}

func DeserializeGhosts(data []byte) (ghosts []ruid.ID, err error) {
	if len(data) <= 0 {
		return
	}
	var ghost ruid.ID
	input := &tygo.ProtoBuf{Buffer: data}
	for !input.ExpectEnd() {
		if ghost, err = server.DeserializeID(input); err != nil {
			return
		} else {
			ghosts = append(ghosts, ghost)
		}
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

func ReserializeNotify(method string, data []byte) (param []byte, err error) {
	param = data
	if method != SYMBOL_NOTIFY {
		return
	}

	input := &tygo.ProtoBuf{Buffer: data}
	var component, property string
	if component, err = input.ReadSymbol(); err != nil {
		return
	} else if property, err = input.ReadSymbol(); err != nil {
		return
	}

	c := GATE_SYMDICT[component]
	p := GATE_SYMDICT[property]
	data = input.Bytes()

	size := tygo.SizeVarint(c) + tygo.SizeVarint(p) + len(data)
	param = make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: param}
	output.WriteVarint(c)
	output.WriteVarint(p)
	output.Write(data)
	return
}

func ReserializeComponents(components [][]byte) (size int, cs []uint64, ds [][]byte, err error) {
	for _, component := range components {
		var name string
		input := &tygo.ProtoBuf{Buffer: component}
		if name, err = input.ReadSymbol(); err != nil {
			return
		} else {
			c := GATE_SYMDICT[name]
			d := input.Bytes()
			cs = append(cs, c)
			ds = append(ds, d)
			size += tygo.SizeVarint(c) + len(d)
		}
	}
	return
}

func SerializeSynchron(i ruid.ID, components [][]byte, handshake []byte) (data []byte, err error) {
	size, cs, ds, err := ReserializeComponents(components)
	if err != nil {
		return
	}
	size += len(handshake) + i.ByteSize()

	data = make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	i.Serialize(output)
	output.Write(handshake)
	for i, c := range cs {
		output.WriteVarint(c)
		output.Write(ds[i])
	}
	return
}
