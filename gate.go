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

func Gate(address string, session string, symbols []string, network Network) {
	SYMBOL_SESSION = session
	serverID := server.ServerID()
	sessionBytes := []byte(SYMBOL_SESSION)
	CREATE_SESSION = make([]byte, 1+len(sessionBytes)+serverID.ByteSize())
	output := &tygo.ProtoBuf{Buffer: CREATE_SESSION}
	output.WriteBytes(1)
	serverID.Serialize(output)
	output.Write(sessionBytes)

	size := 0
	GATE_SYMBOLS = symbols
	for i, symbol := range GATE_SYMBOLS {
		size += tygo.SizeBuffer([]byte(symbol))
		GATE_SYMDICT[symbol] = uint64(i)
	}
	sessionType := GATE_SYMDICT[SYMBOL_SESSION]
	HANDSHAKE_DATA = make([]byte, tygo.SizeVarint(uint64(size))+size+
		serverID.ByteSize()+tygo.SizeVarint(sessionType))
	output = &tygo.ProtoBuf{Buffer: HANDSHAKE_DATA}
	output.WriteVarint(uint64(size))
	for _, symbol := range GATE_SYMBOLS {
		output.WriteBuf([]byte(symbol))
	}
	serverID.Serialize(output)
	output.WriteVarint(sessionType)

	network.Serve(address, GateInst.handler)
}

func (s *GateImpl) handler(gate Connection) {
	session := server.Ident.New()
	SESSION_BYTES := make([]byte, session.ByteSize())
	session.Serialize(&tygo.ProtoBuf{Buffer: SESSION_BYTES})

	if _, err := server.Distribute(session, server.ServerID(), SYMBOL_SESSION, SYMBOL_CREATE, CREATE_SESSION); err != nil {
		log.Printf("[Gate@%v] Create session error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
	} else if components, err := server.Distribute(session, server.ServerID(), SYMBOL_SESSION, SYMBOL_SYNCHRON, nil); err != nil {
		log.Printf("[Gate@%v] Synchron session error %v %v:\n>>>> %v", server.Addr, gate.Address(), session, err)
	} else if handshake, err := SerializeHandshake(session, components); err != nil {
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
			} else if synchron, err := SerializeSynchron(i, components); err != nil {
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
	var component, property []byte
	if component, err = input.ReadBuf(); err != nil {
		return
	} else if property, err = input.ReadBuf(); err != nil {
		return
	}

	c := GATE_SYMDICT[string(component)]
	p := GATE_SYMDICT[string(property)]
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
		var name []byte
		input := &tygo.ProtoBuf{Buffer: component}
		if name, err = input.ReadBuf(); err != nil {
			return
		} else {
			c := GATE_SYMDICT[string(name)]
			d := input.Bytes()
			cs = append(cs, c)
			ds = append(ds, d)
			size += tygo.SizeVarint(c) + len(d)
		}
	}
	return
}

func SerializeSynchron(i ruid.ID, components [][]byte) (data []byte, err error) {
	size, cs, ds, err := ReserializeComponents(components)
	if err != nil {
		return
	}
	size += i.ByteSize()

	data = make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	i.Serialize(output)
	for i, c := range cs {
		output.WriteVarint(c)
		output.Write(ds[i])
	}
	return
}

func SerializeHandshake(i ruid.ID, components [][]byte) (data []byte, err error) {
	size, cs, ds, err := ReserializeComponents(components)
	if err != nil {
		return
	}
	size += len(HANDSHAKE_DATA) + i.ByteSize()

	data = make([]byte, size)
	output := &tygo.ProtoBuf{Buffer: data}
	i.Serialize(output)
	output.Write(HANDSHAKE_DATA)
	for i, c := range cs {
		output.WriteVarint(c)
		output.Write(ds[i])
	}
	return
}
