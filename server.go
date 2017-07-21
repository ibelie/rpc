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
	Address  string
	Services []uint64
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
	Node() *Node
	Add(string, *Node)
	Remove(string)
	Address() string
	Register(...*Node)
	Notify(ruid.RUID, ruid.RUID, []byte) error
	Distribute(ruid.RUID, ruid.RUID, uint64, uint64, []byte, chan<- []byte) error
	Procedure(ruid.RUID, ruid.RUID, uint64, uint64, []byte) ([]byte, error)
	Request(string, ruid.RUID, uint64, uint64, []byte) ([]byte, error)
}

func NewServer(address string, symbols map[string]uint64, routes map[uint64]map[uint64]bool,
	network Network, rs ...Register) Server {
	server = &Server{
		Node:    Node{Address: address},
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
		server.Services = append(server.Services, i)
		server.remote[i] = ruid.NewRing(address)
		server.local[i] = c
	}
	return server
}

func (s *_Server) Notify(i ruid.RUID, k ruid.RUID, p []byte) (err error) {
	_, err = s.Procedure(i, k, SYMBOL_HUB, SYMBOL_NOTIFY, p)
	return
}

func (s *_Server) Distribute(i ruid.RUID, k ruid.RUID, t uint64, m uint64, p []byte, r chan<- []byte) error {
	routes, ok := s.routes[t]
	if !ok {
		return fmt.Errorf("[Distribute] Unknown entity type: %s(%d)", s.symdict[t], t)
	}
	if r != nil {
		go func() {
			for c, ok := range routes {
				if cs, exist := s.routes[m]; exist {
					if ok, exist := cs[c]; !ok || !exist {
						continue
					}
				}
				if !ok {
					continue
				} else if d, e := s.Procedure(i, k, c, m, p); e != nil {
					log.Printf("[MicroServer@%v] Distribute error:\n>>>> %v", s.Address, e)
				} else {
					r <- d
				}
			}
			close(r)
		}()
		return nil
	} else {
		var errors []string
		for c, ok := range routes {
			if cs, exist := s.routes[m]; exist {
				if ok, exist := cs[c]; !ok || !exist {
					continue
				}
			}
			if !ok {
				continue
			} else if _, e := s.Procedure(i, k, c, m, p); e != nil {
				errors = append(errors, fmt.Sprintf("\n>>>> service: %s(%v)\n>>>> %v", s.symdict[c], c, e))
			}
		}
		return fmt.Errorf("[Distribute] %s(%v:%v) %s(%v) errors:%s", s.symdict[t], i, k, s.symdict[m], m, strings.Join(errors, ""))
	}
}

func (s *_Server) Procedure(i ruid.RUID, k ruid.RUID, c uint64, m uint64, p []byte) (r []byte, err error) {
	var node string
	if k == 0 {
		k = i
	}

	if ring, ok := s.remote[c]; !ok {
		err = fmt.Errorf("[Procedure] Unknown service type: %s(%d)", s.symdict[c], c)
		return
	} else if node, ok = ring.Get(k); !ok {
		err = fmt.Errorf("[Procedure] No service found: %s(%d) %v %v", s.symdict[c], c, s.Node, s.nodes)
		return
	}
	r, err = s.Request(node, i, c, m, p)
	return
}

func (s *_Server) Request(node string, i ruid.RUID, c uint64, m uint64, p []byte) (r []byte, err error) {
	// if node == s.Address {
	// 	if service, ok := s.local[c]; !ok {
	// 		err = fmt.Errorf("[Request] No local service found: %s(%d) %v %v", s.symdict[c], c, s.Node, s.local)
	// 	} else {
	// 		r, err = service.Procedure(i, m, p)
	// 	}
	// 	return
	// }

	// if _, ok := s.conns[node]; !ok {
	// 	s.mutex.Lock()
	// 	s.conns[node] = &sync.Pool{New: func() interface{} {
	// 		if conn, err := net.DialTimeout("tcp", node, CONN_DEADLINE*time.Second); err != nil {
	// 			log.Printf("[MicroServer@%v] Connection failed: %s\n>>>> %v", s.Address, node, err)
	// 			return nil
	// 		} else {
	// 			return conn
	// 		}
	// 	}}
	// 	s.mutex.Unlock()
	// }

	// var n int
	// var length uint64
	// var hasLength bool
	// var header [binary.MaxVarintLen64 * 4]byte
	// var buffer [BUFFER_SIZE]byte
	// buflen := binary.PutUvarint(header[:], uint64(i))
	// buflen += binary.PutUvarint(header[buflen:], c)
	// buflen += binary.PutUvarint(header[buflen:], m)
	// buflen += binary.PutUvarint(header[buflen:], uint64(len(p)))
	// param := append(header[:buflen], p...)

	// for j := 0; j < 3; j++ {
	// 	if o := s.conns[node].Get(); o == nil {
	// 		continue
	// 	} else if conn, ok := o.(net.Conn); !ok {
	// 		log.Printf("[MicroServer@%v] Connection pool type error: %v", s.Address, o)
	// 		continue
	// 	} else if err = conn.SetWriteDeadline(time.Now().Add(time.Second * WRITE_DEADLINE)); err != nil {
	// 	} else if _, err = conn.Write(param); err != nil {
	// 	} else {
	// 		var result []byte
	// 		for {
	// 			if n, err = conn.Read(buffer[:]); err != nil {
	// 				if err == io.EOF || isClosedConnError(err) {
	// 					err = fmt.Errorf("[Request] Connection lost:\n>>>> %v", err)
	// 				} else if e, ok := err.(net.Error); ok && e.Timeout() {
	// 					err = fmt.Errorf("[Request] Connection timeout:\n>>>> %v", e)
	// 				} else {
	// 					err = fmt.Errorf("[Request] Connection error:\n>>>> %v", err)
	// 				}
	// 				return
	// 			} else {
	// 				result = append(result, buffer[:n]...)
	// 			}
	// 			if !hasLength {
	// 				length = 0
	// 				var k uint
	// 				for l, b := range result {
	// 					if b < 0x80 {
	// 						if l > 9 || l == 9 && b > 1 {
	// 							err = fmt.Errorf("[Request] Response protocol error: %v %v",
	// 								result[:l], length)
	// 							return
	// 						}
	// 						length |= uint64(b) << k
	// 						result = result[l+1:]
	// 						hasLength = true
	// 					}
	// 					length |= uint64(b&0x7f) << k
	// 					k += 7
	// 				}
	// 			}
	// 			if hasLength && uint64(len(result)) >= length {
	// 				if r = result[:length]; uint64(len(result)) > length {
	// 					log.Printf("[MicroServer@%v] Ignore response data: %v", s.Address, result)
	// 				}
	// 				break
	// 			}
	// 		}
	// 		s.conns[node].Put(conn)
	// 		break
	// 	}

	// 	if err != nil {
	// 		if err == io.EOF || isClosedConnError(err) {
	// 		} else if e, ok := err.(net.Error); ok && e.Timeout() {
	// 		} else {
	// 			log.Printf("[MicroServer@%v] Request retry:\n>>>> %v", s.Address, err)
	// 		}
	// 	}
	// }

	return
}

func (s *_Server) handler(conn Connection) {
	var id ruid.RUID
	var service, method, length, step uint64
	var data []byte
	var lenBuf [binary.MaxVarintLen64]byte
	var buffer [BUFFER_SIZE]byte
	defer conn.Close()
	for {
		conn.SetReadDeadline(time.Now().Add(time.Second * READ_DEADLINE))
		if n, err := conn.Read(buffer[:]); err != nil {
			if err == io.EOF || isClosedConnError(err) {
				log.Printf("[MicroServer@%v] Connection lost:\n>>>> %v", s.Address, err)
			} else if e, ok := err.(net.Error); ok && e.Timeout() {
				log.Printf("[MicroServer@%v] Connection timeout:\n>>>> %v", s.Address, e)
			} else {
				log.Printf("[MicroServer@%v] Connection error:\n>>>> %v", s.Address, err)
			}
			return
		} else {
			data = append(data, buffer[:n]...)
		}
		for step < 4 {
			var x, k uint64
			for i, b := range data {
				if b < 0x80 {
					if i > 9 || i == 9 && b > 1 {
						log.Printf("[MicroServer@%v] Request protocol error: %v %v %s(%v) %s(%v) %v %v",
							s.Address, data[:i], id, s.symdict[service], service, s.symdict[method], method, length, step)
						return // overflow
					}
					x |= uint64(b) << k
					data = data[i+1:]
					switch step {
					case 0:
						id = ruid.RUID(x)
					case 1:
						service = x
					case 2:
						method = x
					case 3:
						length = x
					}
					step++
				}
				x |= uint64(b&0x7f) << k
				k += 7
			}
		}
		if step == 4 && uint64(len(data)) >= length {
			param := data[:length]
			data = data[length:]
			if services, ok := s.local[service]; !ok {
				log.Printf("[MicroServer@%v] Service %s(%d) not exists", s.Address, s.symdict[service], service)
			} else if result, err := services.Procedure(id, method, param); err != nil {
				log.Printf("[MicroServer@%v] Procedure error:\n>>>> %v", s.Address, err)
			} else if _, err := conn.Write(append(lenBuf[:binary.PutUvarint(lenBuf[:], uint64(len(result)))], result...)); err != nil {
				log.Printf("[MicroServer@%v] Response error:\n>>>> %v", s.Address, err)
			}
			step = 0
		}
	}
}

func (s *_Server) Serve() {
	s.Network.Serve(s.Address, s.handler)
}

func (s *_Server) Node() *Node {

}

func (s *_Server) Address() string {

}

func (s *_Server) Register(nodes ...*Node) {
	for _, node := range nodes {
		for _, service := range node.Services {
			if ring, ok := s.remote[service]; ok {
				ring.Append(node.Address)
			} else {
				s.remote[service] = ruid.NewRing(node.Address)
			}
		}
		s.nodes[node.Address] = node
	}
}

func (s *_Server) Add(key string, node *Node) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, service := range node.Services {
		if ring, ok := s.remote[service]; ok {
			ring.Append(node.Address)
		} else {
			s.remote[service] = ruid.NewRing(node.Address)
		}
	}
	s.nodes[key] = node
}

func (s *_Server) Remove(key string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if node, ok := s.nodes[key]; ok {
		for _, service := range node.Services {
			s.remote[service].Remove(node.Address)
		}
	}
	delete(s.nodes, key)
}
