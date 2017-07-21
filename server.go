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

const (
	SYMBOL_SESSION uint64 = iota
	SYMBOL_GATE
	SYMBOL_HUB
	SYMBOL_CREATE
	SYMBOL_DESTROY
	SYMBOL_SYNCHRON
	SYMBOL_NOTIFY
	SYMBOL_OBSERVE
	SYMBOL_IGNORE
)

var BUILTIN_SYMBOLS = []string{
	"Session",
	"GATE",
	"HUB",
	"CREATE",
	"DESTROY",
	"SYNCHRON",
	"NOTIFY",
	"OBSERVE",
	"IGNORE",
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
	Srvs []uint64
}

type _Server struct {
	Node
	Network
	mutex   sync.Mutex
	routes  map[uint64]map[uint64]bool
	symbols map[string]uint64
	symdict map[uint64]string
	nodes   map[string]*Node
	conns   map[string]*sync.Pool
	remote  map[uint64]*ruid.Ring
	local   map[uint64]Service
}

var server *_Server

type Service interface {
	Procedure(ruid.RUID, uint64, []byte) ([]byte, error)
}

type Register func(Server, map[string]uint64) (uint64, Service)

type Server interface {
	Serve()
	GetNode() *Node
	Add(string, *Node)
	Remove(string)
	Address() string
	Register(...*Node)
	Notify(ruid.RUID, ruid.RUID, []byte) error
	Distribute(ruid.RUID, ruid.RUID, uint64, uint64, []byte, chan<- []byte) error
	Procedure(ruid.RUID, ruid.RUID, uint64, uint64, []byte) ([]byte, error)
	Request(string, ruid.RUID, uint64, []byte, ...uint64) ([][]byte, error)
}

func NewServer(address string, symbols map[string]uint64, routes map[uint64]map[uint64]bool,
	network Network, rs ...Register) Server {
	server = &_Server{
		Node:    Node{Addr: address},
		Network: network,
		routes:  routes,
		symbols: symbols,
		symdict: make(map[uint64]string),
		nodes:   make(map[string]*Node),
		conns:   make(map[string]*sync.Pool),
		remote:  make(map[uint64]*ruid.Ring),
		local:   make(map[uint64]Service),
	}

	for symbol, value := range symbols {
		server.symdict[value] = symbol
	}

	for _, r := range rs {
		i, c := r(server, symbols)
		server.Srvs = append(server.Srvs, i)
		server.remote[i] = ruid.NewRing(address)
		server.local[i] = c
	}
	return server
}

func (s *_Server) Notify(i ruid.RUID, k ruid.RUID, p []byte) (err error) {
	_, err = s.Procedure(i, k, SYMBOL_HUB, SYMBOL_NOTIFY, p)
	return
}

func (s *_Server) Distribute(i ruid.RUID, k ruid.RUID, t uint64, m uint64, p []byte, r chan<- []byte) (err error) {
	var errors []string
	services := make(map[string][]uint64)

	if routes, ok := s.routes[t]; !ok {
		return fmt.Errorf("[Distribute] Unknown entity type: %s(%v)", s.symdict[t], t)
	} else {
		for c, ok := range routes {
			if !ok {
				continue
			} else if cs, exist := s.routes[m]; exist {
				if ok, exist := cs[c]; !ok || !exist {
					continue
				}
			}
			if ring, ok := s.remote[c]; !ok {
				errors = append(errors, fmt.Sprintf("\n>>>> Unknown service type: %s(%v)", s.symdict[c], c))
			} else if node, ok := ring.Get(k); !ok {
				errors = append(errors, fmt.Sprintf("\n>>>> No service found: %s(%v) %v %v", s.symdict[c], c, s.Node, s.nodes))
			} else {
				services[node] = append(services[node], c)
			}
		}
	}
	if r == nil {
		for node, c := range services {
			if _, e := s.Request(node, i, m, p, c...); e != nil {
				errors = append(errors, fmt.Sprintf("\n>>>> Request error: %v %v\n>>>> %v", node, c, e))
			}
		}
	} else if len(errors) == 0 {
		go func() {
			for node, c := range services {
				if rs, e := s.Request(node, i, m, p, c...); e != nil {
					log.Printf("[Server@%v] Distribute error: %v %v\n>>>> %v", s.Addr, node, c, e)
				} else {
					for _, d := range rs {
						r <- d
					}
				}
			}
			close(r)
		}()
	}
	if len(errors) > 0 {
		err = fmt.Errorf("[Distribute] %s(%v:%v) %s(%v) errors:%s", s.symdict[t], i, k, s.symdict[m], m, strings.Join(errors, ""))
	}
	return
}

func (s *_Server) Procedure(i ruid.RUID, k ruid.RUID, c uint64, m uint64, p []byte) (r []byte, err error) {
	if k == 0 {
		k = i
	}

	if ring, ok := s.remote[c]; !ok {
		err = fmt.Errorf("[Procedure] Unknown service type: %s(%v)", s.symdict[c], c)
	} else if node, ok := ring.Get(k); !ok {
		err = fmt.Errorf("[Procedure] No service found: %s(%v) %v %v", s.symdict[c], c, s.Node, s.nodes)
	} else if rs, e := s.Request(node, i, m, p, c); e != nil || len(rs) <= 0 {
		err = e
	} else {
		r = rs[0]
	}
	return
}

func (s *_Server) Request(node string, i ruid.RUID, m uint64, p []byte, cs ...uint64) (rs [][]byte, err error) {
	var errors []string
	if node == s.Addr {
		for _, c := range cs {
			if service, ok := s.local[c]; !ok {
				errors = append(errors, fmt.Sprintf("\n>>>> No local service found: %s(%v) %v %v", s.symdict[c], c, s.Node, s.local))
			} else if r, e := service.Procedure(i, m, p); e != nil {
				errors = append(errors, fmt.Sprintf("\n>>>> Procedure %s(%v) error\n>>>> %v", s.symdict[c], c, e))
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
		err = fmt.Errorf("[Request] %s %v %s(%v) errors:%s", node, i, s.symdict[m], m, strings.Join(errors, ""))
	}
	return
}

func (s *_Server) handler(conn Connection) {
	defer conn.Close()
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
					log.Printf("[Server@%v] Service %s(%v) not exists", s.Addr, s.symdict[c], c)
				} else if r, err := service.Procedure(i, m, p); err != nil {
					log.Printf("[Server@%v] Procedure %s(%v) error:\n>>>> %v", s.Addr, s.symdict[c], c, err)
				} else {
					rs = append(rs, r)
				}
			}
			var size int
			for _, r := range rs {
				size += tygo.SizeVarint(uint64(len(r))) + len(r)
			}
			result := make([]byte, size)
			output := &tygo.ProtoBuf{Buffer: result}
			for _, r := range rs {
				output.WriteVarint(uint64(len(r)))
				output.Write(r)
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
				s.remote[service] = ruid.NewRing(node.Addr)
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
			s.remote[service] = ruid.NewRing(node.Addr)
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

func SerializeRequest(i ruid.RUID, ss []uint64, m uint64, p []byte) (data []byte) {
	var size int
	for _, s := range ss {
		size += tygo.SizeVarint(s)
	}
	data = make([]byte, tygo.SizeVarint(uint64(i))+tygo.SizeVarint(m)+tygo.SizeVarint(uint64(size))+size+len(p))
	output := &tygo.ProtoBuf{Buffer: data}
	output.WriteVarint(uint64(i))
	output.WriteVarint(m)
	output.WriteVarint(uint64(size))
	for _, s := range ss {
		output.WriteVarint(s)
	}
	output.Write(p)
	return
}

func DeserializeRequest(data []byte) (i ruid.RUID, ss []uint64, m uint64, p []byte, err error) {
	var id uint64
	input := &tygo.ProtoBuf{Buffer: data}
	if id, err = input.ReadVarint(); err != nil {
	} else if m, err = input.ReadVarint(); err != nil {
	} else if p, err = input.ReadBuf(); err != nil {
	} else {
		i = ruid.RUID(id)
		buffer := &tygo.ProtoBuf{Buffer: p}
		for !buffer.ExpectEnd() {
			if s, e := buffer.ReadVarint(); e != nil {
				err = e
				break
			} else {
				ss = append(ss, s)
			}
		}
		p = input.Bytes()
	}
	return
}
