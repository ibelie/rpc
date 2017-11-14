// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"sort"
	"strings"

	"github.com/ibelie/rpc/strid"
	"github.com/ibelie/rpc/uuid"
	"github.com/ibelie/ruid"
)

var JSID_ZERO = []string{
	ruid.ZERO.String(),
	uuid.ZERO.String(),
	strid.ZERO.String(),
}

var JSID_REQUIRE = []string{
	"",
	"",
	`
	goog.require('ibelie.tyts.String');`,
}

var JSID_BYTESIZE = []func(string) string{
	func(value string) string {
		return fmt.Sprintf("8")
	},
	func(value string) string {
		return fmt.Sprintf("16")
	},
	func(value string) string {
		return fmt.Sprintf("ibelie.tyts.String.ByteSize(%s, 0, false)", value)
	},
}

var JSID_WRITE = []func(string, string) string{
	func(protobuf string, value string) string {
		return fmt.Sprintf("%s.WriteBase64(%s)", protobuf, value)
	},
	func(protobuf string, value string) string {
		return fmt.Sprintf("%s.WriteBase64(%s)", protobuf, value)
	},
	func(protobuf string, value string) string {
		return fmt.Sprintf("ibelie.tyts.String.Serialize(%s, 0, false, %s)", value, protobuf)
	},
}

var JSID_READ = []func(string) string{
	func(protobuf string) string {
		return fmt.Sprintf("%s.ReadBase64(8)", protobuf)
	},
	func(protobuf string) string {
		return fmt.Sprintf("%s.ReadBase64(16)", protobuf)
	},
	func(protobuf string) string {
		return fmt.Sprintf("ibelie.tyts.String.Deserialize(null, %s)", protobuf)
	},
}

func injectJavascript(identName string, dir string, entities []*Entity) {
	ident := IDENT_FromString(identName)
	var buffer bytes.Buffer
	requireMap := make(map[string]bool)
	methodsMap := make(map[string]bool)

	var methods []string
	for _, e := range entities {
		for _, c := range e.Components {
			if c.Protocol == nil {
				continue
			}
			for _, m := range c.Protocol.Methods {
				if ok, exist := methodsMap[m.Name]; exist && ok {
					continue
				} else if len(m.Results) > 0 {
					continue
				}
				var params []string
				for i, _ := range m.Params {
					params = append(params, fmt.Sprintf("a%d", i))
				}
				localParams := append([]string{"v"}, params...)
				methods = append(methods, fmt.Sprintf(`
Entity.prototype['D_%s'] = function(data) {
	return ibelie.rpc['%s']['D_%sParam'](data);
};

Entity.prototype.%s = function(%s) {
	if (!this.isAwake) {
		console.warn('[Entity] Not awake:', this);
		return;
	}
	for (var k in this) {
		var v = this[k];
		v.%s && v.%s.call(%s);
	}
	var data = ibelie.rpc['%s']['S_%sParam'](%s);
	this.connection.send(this, this.connection.SymDict.%s, data);
};
`, m.Name, c.Name, m.Name, m.Name, strings.Join(params, ", "),
					m.Name, m.Name, strings.Join(localParams, ", "),
					c.Name, m.Name, strings.Join(params, ", "), m.Name))
				requireMap[fmt.Sprintf(`
goog.require('ibelie.rpc.%s');`, c.Name)] = true
				methodsMap[m.Name] = true
			}
		}
	}

	var requires []string
	for require, ok := range requireMap {
		if ok {
			requires = append(requires, require)
		}
	}
	sort.Strings(requires)

	buffer.Write([]byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

goog.provide('ibelie.rpc.Entity');
goog.provide('ibelie.rpc.Connection');
%s
goog.require('ibelie.tyts.ProtoBuf');
goog.require('ibelie.tyts.SizeVarint');
goog.require('ibelie.tyts.SymbolEncodedLen');%s

var ZERO_ID = %q;

ibelie.rpc.Entity = function() {
	this.__class__ = 'Entity';
	this.isAwake = false;
	this.ID = ZERO_ID;
	this.Key = ZERO_ID;
	this.Type = '';
};

ibelie.rpc.Entity.prototype.ByteSize = function() {
	var size = 1 + ibelie.tyts.SymbolEncodedLen(this.Type);
	if (this.ID != ZERO_ID) {
		size += %s;
	}
	if (this.Key != ZERO_ID) {
		size += %s;
	}
	return size;
};

ibelie.rpc.Entity.prototype.Serialize = function() {
	var protobuf = new ibelie.tyts.ProtoBuf(new Uint8Array(this.ByteSize()));
	var t = 0;
	if (this.ID != ZERO_ID) {
		t |= 1;
	}
	if (this.Key != ZERO_ID) {
		t |= 2;
	}
	protobuf.WriteByte(t);
	if (this.ID != ZERO_ID) {
		%s;
	}
	if (this.Key != ZERO_ID) {
		%s;
	}
	protobuf.EncodeSymbol(this.Type);
	return protobuf.buffer;
};

ibelie.rpc.Entity.prototype.Deserialize = function(data) {
	var protobuf = new ibelie.tyts.ProtoBuf(data);
	var t = protobuf.ReadByte();
	this.ID = (t & 1) ? %s : ZERO_ID;
	this.Key = (t & 2) ? %s : ZERO_ID;
	this.Type = protobuf.DecodeSymbol();
};

ibelie.rpc.Entity.prototype.Awake = function(e) {
	if (!e) {
		console.warn('[Entity] No entity:', e);
		return e;
	} else if (e.isAwake) {
		console.warn('[Entity] Already awaked:', e);
		return e;
	}
	var conn = this.connection;
	var entity = conn.entities[e.ID];
	if (entity) {
		return entity
	}
	entity = new ibelie.rpc.entities[e.Type]();
	entity.ID = e.ID;
	entity.Key = e.Key;
	entity.Type = e.Type;
	entity.connection = conn;
	conn.send(e, conn.SymDict.OBSERVE);
	conn.entities[entity.ID] = entity;
	return entity;
};

ibelie.rpc.Entity.prototype.Drop = function(e) {
	if (!e || !e.isAwake) {
		console.warn('[Entity] Not awaked:', e);
		return;
	}
	for (var k in e) {
		var v = e[k];
		v.onDrop && v.onDrop();
		if (v.Entity) {
			delete v.Entity;
		}
	}
	e.isAwake = false;
	var conn = this.connection;
	conn.send(e, conn.SymDict.IGNORE);
	delete conn.entities[e.ID];
	var entity = new Entity();
	entity.ID = e.ID;
	entity.Key = e.Key;
	entity.Type = e.Type;
	return entity;
};

ibelie.rpc.Connection = function(url) {
	var conn = this;
	var socket = new WebSocket(url);
	socket.onopen = function (event) {
		socket.onmessage = function(event) {
			var entity;
			var protobuf = ibelie.tyts.ProtoBuf.FromBase64(event.data);
			var id = %s;
			if (!conn.Symbols) {
				conn.Symbols = [];
				conn.SymDict = {};
				var buffer = new ibelie.tyts.ProtoBuf(protobuf.ReadBuffer());
				var value = 0;
				while (!buffer.End()) {
					var symbol = buffer.ReadSymbol();
					conn.Symbols = conn.Symbols.concat(symbol);
					conn.SymDict[symbol] = value;
					value++;
				}
				var key = %s;
				var t = conn.Symbols[protobuf.ReadVarint()];
				entity = new ibelie.rpc.entities[t]();
				entity.connection = conn;
				entity.ID = id;
				entity.Key = key;
				entity.Type = t;
				conn.entities[id] = entity;
			} else {
				entity = conn.entities[id];
				if (!entity) {
					console.error('[Connection] Cannot find entity:', id);
					return;
				}
			}
			while (!protobuf.End()) {
				var name = conn.Symbols[protobuf.ReadVarint()];
				var data = protobuf.ReadBuffer();
				if (ibelie.rpc[name]) {
					ibelie.rpc[name].prototype.Deserialize.call(entity[name], data);
				} else if (!entity.isAwake) {
					console.error('[Connection] Entity is not awake:', id, name, entity);
					continue;
				} else if (name == 'NOTIFY') {
					var buffer = new ibelie.tyts.ProtoBuf(data);
					var compName = conn.Symbols[buffer.ReadVarint()];
					var property = conn.Symbols[buffer.ReadVarint()];
					var newValue = ibelie.rpc[compName]['D_' + property](buffer.Bytes())[0];
					var component = entity[compName];
					var oldValue = component[property];
					var handler = component[property + 'Handler'];
					if (oldValue.concat) {
						component[property] = oldValue.concat(newValue);
						handler && handler.call(component, oldValue, newValue);
					} else if ((newValue instanceof Object) && !newValue.__class__) {
						if (!component[property]) {
							component[property] = {};
						}
						for (var k in newValue) {
							var o = oldValue[k];
							var n = newValue[k];
							oldValue[k] = n;
							handler && handler.call(component, k, o, n);
						}
					} else {
						component[property] = newValue;
						handler && handler.call(component, oldValue, newValue);
					}
				} else {
					var args = entity['D_' + name](data);
					for (var k in entity) {
						var v = entity[k];
						v[name] && v[name].apply(v, args);
					}
				}
			}
			if (entity && !entity.isAwake) {
				entity.isAwake = true;
				for (var k in entity) {
					var v = entity[k];
					v.onAwake && v.onAwake();
				}
			}
		};
		socket.onclose = function(event) {
			console.warn('[Connection] Socket has been closed:', event, conn);
		};
	};
	this.socket = socket;
	this.entities = {};
};

ibelie.rpc.Connection.prototype.send = function(entity, method, data) {
	var t = this.SymDict[entity.Type] << 2;
	var size = ibelie.tyts.SizeVarint(t) + ibelie.tyts.SizeVarint(method);
	if (entity.ID != ZERO_ID) {
		size += %s;
		t |= 1;
	}
	if (entity.Key != ZERO_ID) {
		size += %s;
		t |= 2;
	}
	if (data) {
		size += data.length;
	}
	var protobuf = new ibelie.tyts.ProtoBuf(new Uint8Array(size));
	protobuf.WriteVarint(t);
	if (entity.ID != ZERO_ID) {
		%s;
	}
	if (entity.Key != ZERO_ID) {
		%s;
	}
	protobuf.WriteVarint(method);
	if (data) {
		protobuf.WriteBytes(data);
	}
	this.socket.send(protobuf.ToBase64());
};

ibelie.rpc.Connection.prototype.disconnect = function() {
	this.socket.close();
};
%s`, JSID_REQUIRE[ident], strings.Join(requires, ""), JSID_ZERO[ident],
		JSID_BYTESIZE[ident]("this.ID"), JSID_BYTESIZE[ident]("this.Key"),
		JSID_WRITE[ident]("protobuf", "this.ID"), JSID_WRITE[ident]("protobuf", "this.Key"),
		JSID_READ[ident]("protobuf"), JSID_READ[ident]("protobuf"),
		JSID_READ[ident]("protobuf"), JSID_READ[ident]("protobuf"),
		JSID_BYTESIZE[ident]("this.ID"), JSID_BYTESIZE[ident]("this.Key"),
		JSID_WRITE[ident]("protobuf", "entity.ID"), JSID_WRITE[ident]("protobuf", "entity.Key"),
		strings.Join(methods, ""))))

	ioutil.WriteFile(path.Join(dir, "rpc.js"), buffer.Bytes(), 0666)
}
