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

const CLIENT_ENTITY_FMT = "%sClient"

const (
	SYMBOL_GATE     = "GATE"
	SYMBOL_HUB      = "HUB"
	SYMBOL_CREATE   = "CREATE"
	SYMBOL_DESTROY  = "DESTROY"
	SYMBOL_SYNCHRON = "SYNCHRON"
	SYMBOL_NOTIFY   = "NOTIFY"
	SYMBOL_OBSERVE  = "OBSERVE"
	SYMBOL_IGNORE   = "IGNORE"
)

var BUILTIN_SYMBOLS = []string{
	SYMBOL_GATE,
	SYMBOL_HUB,
	SYMBOL_CREATE,
	SYMBOL_DESTROY,
	SYMBOL_SYNCHRON,
	SYMBOL_NOTIFY,
	SYMBOL_OBSERVE,
	SYMBOL_IGNORE,
}

type Connection interface {
	Address() string
	Send([]byte) error
	Receive() ([]byte, error)
	Close() error
}

type Network interface {
	Connect(string) Connection
	Serve(string, func(Connection))
}

type Node struct {
	Addr string
	Srvs []string
}

type _Server struct {
	Node
	Network
	ruid.Ident
	mutex  sync.Mutex
	routes map[string]map[string]bool
	nodes  map[string]*Node
	conns  map[string]*sync.Pool
	remote map[string]*ruid.Ring
	local  map[string]Service
}

var server *_Server

type Service interface {
	Procedure(ruid.ID, string, []byte) ([]byte, error)
}

type Register func(Server) (string, Service)

type Server interface {
	Serve()
	GetNode() *Node
	Add(string, *Node)
	Remove(string)
	Address() string
	Register(...*Node)
	ZeroID() ruid.ID
	ServerID() ruid.ID
	DeserializeID(*tygo.ProtoBuf) (ruid.ID, error)
	Notify(ruid.ID, ruid.ID, string, []byte) error
	Message(ruid.ID, ruid.ID, ruid.ID, string, []byte) ([]byte, error)
	Distribute(ruid.ID, ruid.ID, string, string, []byte) ([][]byte, error)
	Procedure(ruid.ID, ruid.ID, string, string, string, []byte) ([]byte, error)
	Request(string, ruid.ID, string, []byte, ...string) ([][]byte, error)
}

func NewServer(address string, routes map[string]map[string]bool,
	ident ruid.Ident, network Network, rs ...Register) Server {
	server = &_Server{
		Node:    Node{Addr: address},
		Ident:   ident,
		Network: network,
		routes:  routes,
		nodes:   make(map[string]*Node),
		conns:   make(map[string]*sync.Pool),
		remote:  make(map[string]*ruid.Ring),
		local:   make(map[string]Service),
	}

	for _, r := range rs {
		i, c := r(server)
		server.Srvs = append(server.Srvs, i)
		server.remote[i] = ruid.NewRing(ident, address)
		server.local[i] = c
	}
	return server
}

func (s *_Server) ZeroID() ruid.ID {
	return s.Ident.Zero()
}

func (s *_Server) ServerID() ruid.ID {
	return ruid.RingKey(s.Ident, s.Addr)
}

func (s *_Server) DeserializeID(input *tygo.ProtoBuf) (ruid.ID, error) {
	return s.Ident.Deserialize(input)
}

func (s *_Server) Notify(i ruid.ID, k ruid.ID, t string, p []byte) (err error) {
	_, err = s.Procedure(i, k, t, SYMBOL_HUB, SYMBOL_NOTIFY, p)
	return
}

var ClientEntityCache = make(map[string]string)

func (s *_Server) Distribute(i ruid.ID, k ruid.ID, t string, m string, p []byte) (rs [][]byte, err error) {
	if !k.Nonzero() {
		k = i
	}

	var errors []string
	var components []string

	if routes, ok := s.routes[t]; !ok {
		err = fmt.Errorf("[Distribute] Unknown entity type %q", t)
		return
	} else if cs, exist := s.routes[m]; !exist {
		for c, ok := range routes {
			if ok {
				components = append(components, c)
			}
		}
	} else {
		for c, ok := range routes {
			if !ok {
				continue
			} else if ok, exist := cs[c]; ok && exist {
				components = append(components, c)
			}
		}

		var clientEntity string
		if name, ok := ClientEntityCache[t]; ok {
			clientEntity = name
		} else {
			clientEntity = fmt.Sprintf(CLIENT_ENTITY_FMT, t)
			ClientEntityCache[t] = clientEntity
		}
		if ok, exist := cs[clientEntity]; ok && exist {
			if t == SYMBOL_SESSION {
				components = append(components, SYMBOL_GATE)
			} else {
				components = append(components, SYMBOL_HUB)
			}
		}
	}

	services := make(map[string][]string)
	for _, c := range components {
		if ring, ok := s.remote[c]; !ok {
			errors = append(errors, fmt.Sprintf("\n>>>> Unknown service type %q", c))
		} else if node, ok := ring.Get(k); !ok {
			errors = append(errors, fmt.Sprintf("\n>>>> No service %q found: %v %v", c, s.Node, s.nodes))
		} else if c != SYMBOL_GATE {
			services[node] = append(services[node], c)
		} else if _, err = s.Request(node, i, m, SerializeDispatch(map[ruid.ID]bool{i: true}, p), c); err != nil {
			errors = append(errors, fmt.Sprintf("\n>>>> Dispatch session gate: %v\n>>>> %v", node, err))
		}
	}

	for node, c := range services {
		if ds, e := s.Request(node, i, m, p, c...); e != nil {
			errors = append(errors, fmt.Sprintf("\n>>>> Request error: %v %v\n>>>> %v", node, c, e))
		} else {
			rs = append(rs, ds...)
		}
	}

	if len(errors) > 0 {
		err = fmt.Errorf("[Distribute] %s(%v:%v) %q errors:%s", t, i, k, m, strings.Join(errors, ""))
	}
	return
}

func (s *_Server) Message(i ruid.ID, k ruid.ID, t ruid.ID, m string, p []byte) (r []byte, err error) {
	if ring, ok := s.remote[SYMBOL_GATE]; !ok {
		err = fmt.Errorf("[Message] Cannot find gate service: %v %v %v", s.remote, s.Node, s.nodes)
	} else if node, ok := ring.Get(k); !ok {
		err = fmt.Errorf("[Message] No gate found: %v %v", s.Node, s.nodes)
	} else if rs, e := s.Request(node, t, m, SerializeDispatch(map[ruid.ID]bool{i: true}, p), SYMBOL_GATE); e != nil || len(rs) <= 0 {
		err = e
	} else {
		r = rs[0]
	}
	return
}

func (s *_Server) Procedure(i ruid.ID, k ruid.ID, t string, c string, m string, p []byte) (r []byte, err error) {
	if !k.Nonzero() {
		k = i
	}
	if t == SYMBOL_SESSION && c == SYMBOL_HUB {
		c = SYMBOL_GATE
		p = SerializeDispatch(map[ruid.ID]bool{i: true}, p)
	}

	if ring, ok := s.remote[c]; !ok {
		err = fmt.Errorf("[Procedure] Unknown service type %q", c)
	} else if node, ok := ring.Get(k); !ok {
		err = fmt.Errorf("[Procedure] No service %q found: %v %v", c, s.Node, s.nodes)
	} else if rs, e := s.Request(node, i, m, p, c); e != nil || len(rs) <= 0 {
		err = e
	} else {
		r = rs[0]
	}
	return
}

func (s *_Server) Request(node string, i ruid.ID, m string, p []byte, cs ...string) (rs [][]byte, err error) {
	var errors []string
	if node == s.Addr {
		for _, c := range cs {
			if service, ok := s.local[c]; !ok {
				errors = append(errors, fmt.Sprintf("\n>>>> No local service %q found: %v %v", c, s.Node, s.local))
			} else if r, e := service.Procedure(i, m, p); e != nil {
				errors = append(errors, fmt.Sprintf("\n>>>> Procedure %q of Service %q error\n>>>> %v", m, c, e))
			} else {
				rs = append(rs, r)
			}
		}
		goto request_end
	}

	if _, ok := s.conns[node]; !ok {
		s.mutex.Lock()
		s.conns[node] = &sync.Pool{New: func() interface{} {
			return s.Network.Connect(node)
		}}
		s.mutex.Unlock()
	}

	for j := 0; j < 10; j++ {
		if o := s.conns[node].Get(); o == nil {
			continue
		} else if conn, ok := o.(Connection); !ok {
			log.Printf("[Server@%v] Connection pool type error: %v", s.Addr, o)
			continue
		} else if e := conn.Send(SerializeRequest(i, cs, m, p)); e != nil {
			errors = append(errors, fmt.Sprintf("\n>>>> Connection retry: %d\n>>>> %v", j, e))
		} else {
			ds, e := conn.Receive()
			if e != nil {
				err = fmt.Errorf("[Request] Receive response error:\n>>>> %v", err)
				return
			}
			s.conns[node].Put(conn)
			input := &tygo.ProtoBuf{Buffer: ds}
			for !input.ExpectEnd() {
				if r, e := input.ReadBuf(); e != nil {
					err = fmt.Errorf("[Request] Response deserialize error:\n>>>> %v", e)
					return
				} else {
					rs = append(rs, r)
				}
			}
			return
		}
	}

request_end:
	if len(errors) > 0 {
		err = fmt.Errorf("[Request] %s %v %q errors:%s", node, i, m, strings.Join(errors, ""))
	}
	return
}

func (s *_Server) handler(conn Connection) {
	for {
		if data, err := conn.Receive(); err != nil {
			log.Printf("[Server@%v] Connection error:\n>>>> %v", s.Addr, err)
			break
		} else if i, cs, m, p, err := DeserializeRequest(data); err != nil {
			log.Printf("[Server@%v] Deserialize request error:\n>>>> %v", s.Addr, err)
		} else {
			var rs [][]byte
			for _, c := range cs {
				if service, ok := s.local[c]; !ok {
					log.Printf("[Server@%v] Service %q not exists", s.Addr, c)
				} else if r, err := service.Procedure(i, m, p); err != nil {
					log.Printf("[Server@%v] Procedure %q error:\n>>>> %v", s.Addr, c, err)
				} else {
					rs = append(rs, r)
				}
			}
			var size int
			for _, r := range rs {
				size += tygo.SizeBuffer(r)
			}
			result := make([]byte, size)
			output := &tygo.ProtoBuf{Buffer: result}
			for _, r := range rs {
				output.WriteBuf(r)
			}
			if err := conn.Send(result); err != nil {
				log.Printf("[Server@%v] Response error:\n>>>> %v", s.Addr, err)
			}
		}
	}
}

func (s *_Server) Serve() {
	s.Network.Serve(s.Addr, s.handler)
}

func (s *_Server) GetNode() *Node {
	return &s.Node
}

func (s *_Server) Address() string {
	return s.Addr
}

func (s *_Server) Register(nodes ...*Node) {
	for _, node := range nodes {
		for _, service := range node.Srvs {
			if ring, ok := s.remote[service]; ok {
				ring.Append(node.Addr)
			} else {
				s.remote[service] = ruid.NewRing(s.Ident, node.Addr)
			}
		}
		s.nodes[node.Addr] = node
	}
}

func (s *_Server) Add(key string, node *Node) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, service := range node.Srvs {
		if ring, ok := s.remote[service]; ok {
			ring.Append(node.Addr)
		} else {
			s.remote[service] = ruid.NewRing(s.Ident, node.Addr)
		}
	}
	s.nodes[key] = node
}

func (s *_Server) Remove(key string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if node, ok := s.nodes[key]; ok {
		for _, service := range node.Srvs {
			s.remote[service].Remove(node.Addr)
		}
	}
	delete(s.nodes, key)
}

func SerializeRequest(i ruid.ID, ss []string, m string, p []byte) (data []byte) {
	var size int
	for _, s := range ss {
		size += tygo.SizeBuffer([]byte(s))
	}
	data = make([]byte, i.ByteSize()+tygo.SizeVarint(uint64(size))+size+
		tygo.SizeBuffer([]byte(m))+len(p))
	output := &tygo.ProtoBuf{Buffer: data}
	i.Serialize(output)
	output.WriteVarint(uint64(size))
	for _, s := range ss {
		output.WriteBuf([]byte(s))
	}
	output.WriteBuf([]byte(m))
	output.Write(p)
	return
}

func DeserializeRequest(data []byte) (i ruid.ID, ss []string, m string, p []byte, err error) {
	input := &tygo.ProtoBuf{Buffer: data}
	if i, err = server.DeserializeID(input); err != nil {
	} else if p, err = input.ReadBuf(); err != nil {
	} else {
		buffer := &tygo.ProtoBuf{Buffer: p}
		for !buffer.ExpectEnd() {
			if s, e := buffer.ReadVarint(); e != nil {
				err = e
				return
			} else {
				ss = append(ss, s)
			}
		}
		if p, err = input.ReadBuf(); err == nil {
			m = string(p)
			p = input.Bytes()
		}
	}
	return
}
