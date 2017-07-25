// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"io/ioutil"
)

func injectJavascript(dir string, entities []*Entity) {
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
Entity.prototype.Deserialize%s = function(data) {
	return ibelie.rpc.%s.Deserialize%sParam(data);
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
	var data = ibelie.rpc.%s.Serialize%sParam(%s);
	this.connection.send(this, ibelie.rpc.Symbols.%s, data);
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

goog.provide('Entity');

goog.require('tyts.String');
goog.require('tyts.ProtoBuf');
goog.require('tyts.SizeVarint');%s

var ZERO_RUID = 'AAAAAAAAAAA';

Entity = function() {
	this.__class__ = 'Entity';
	this.isAwake = false;
	this.RUID = ZERO_RUID;
	this.Key  = ZERO_RUID;
	this.Type = 0;
};

Entity.prototype.ByteSize = function() {
	var size = tyts.SizeVarint(this.Type << 2);
	if (this.RUID != ZERO_RUID) {
		size += 8;
	}
	if (this.Key != ZERO_RUID) {
		size += 8;
	}
	return size;
};

Entity.prototype.SerializeUnsealed = function(protobuf) {
	var t = this.Type << 2;
	if (this.RUID != ZERO_RUID) {
		t |= 1;
	}
	if (this.Key != ZERO_RUID) {
		t |= 2;
	}
	protobuf.WriteVarint(t);
	if (this.RUID != ZERO_RUID) {
		protobuf.WriteBase64(this.RUID);
	}
	if (this.Key != ZERO_RUID) {
		protobuf.WriteBase64(this.Key);
	}
};

Entity.prototype.Serialize = function() {
	var protobuf = new tyts.ProtoBuf(new Uint8Array(this.ByteSize()));
	this.SerializeUnsealed(protobuf);
	return protobuf.buffer;
};

Entity.prototype.Deserialize = function(data) {
	var protobuf = new tyts.ProtoBuf(data);
	var t = protobuf.ReadVarint();
	this.Type = t >>> 2;
	this.RUID = (t & 1) ? protobuf.ReadBase64(8) : ZERO_RUID;
	this.Key  = (t & 2) ? protobuf.ReadBase64(8) : ZERO_RUID;
};

var ibelie = {};
ibelie.rpc = {};
ibelie.rpc.Entity = Entity;

ibelie.rpc.Component = function(entity) {
	this.Entity = entity;
};

ibelie.rpc.Component.prototype.Awake = function(e) {
	if (e.isAwake) {
		console.warn('[Entity] Already awaked:', e);
		return e;
	}
	var conn = this.Entity.connection;
	var entity = conn.entities[e.RUID];
	if (entity) {
		return entity
	}
	entity = new entities[ibelie.rpc.Dictionary[e.Type]]();
	entity.RUID	= e.RUID;
	entity.Key	= e.Key;
	entity.Type	= e.Type;
	entity.connection = conn;
	conn.send(e, ibelie.rpc.Symbols.OBSERVE);
	conn.entities[entity.RUID] = entity;
	return entity;
};

ibelie.rpc.Component.prototype.Drop = function(e) {
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
	var conn = this.Entity.connection;
	conn.send(e, ibelie.rpc.Symbols.IGNORE);
	delete conn.entities[e.RUID];
	var entity = new Entity();
	entity.RUID	= e.RUID;
	entity.Key	= e.Key;
	entity.Type	= e.Type;
	return entity;
};

ibelie.rpc.Connection = function(url) {
	var conn = this;
	var socket = new WebSocket(url);
	socket.onopen = function (event) {
		socket.onmessage = function(event) {
			var entity;
			var protobuf = tyts.ProtoBuf.FromBase64(event.data);
			var id = protobuf.ReadBase64(8);
			if (!ibelie.rpc.Symbols) {
				ibelie.rpc.Symbols = {};
				ibelie.rpc.Dictionary = {};
				var buffer = new tyts.ProtoBuf(protobuf.ReadBuffer());
				while (!buffer.End()) {
					var symbol = tyts.String.Deserialize(null, buffer);
					var value = buffer.ReadVarint();
					ibelie.rpc.Symbols[symbol] = value;
					ibelie.rpc.Dictionary[value] = symbol;
				}
				var t = protobuf.ReadVarint();
				entity = new entities[ibelie.rpc.Dictionary[t]]();
				entity.connection = conn;
				entity.Type = t;
				entity.RUID = id;
				entity.Key  = ZERO_RUID;
				conn.entities[id] = entity;
			} else {
				entity = conn.entities[id];
				if (!entity) {
					console.error('[Connection] Cannot find entity:', id);
					return;
				}
			}
			while (!protobuf.End()) {
				var name = ibelie.rpc.Dictionary[protobuf.ReadVarint()];
				var data = protobuf.ReadBuffer();
				if (ibelie.rpc[name]) {
					ibelie.rpc[name].prototype.Deserialize.call(entity[name], data);
				} else if (!entity.isAwake) {
					console.error('[Connection] Entity is not awake:', id, name, entity);
					continue;
				} else if (name == 'NOTIFY') {
					var buffer = new tyts.ProtoBuf(data);
					var compName = ibelie.rpc.Dictionary[buffer.ReadVarint()];
					var property = ibelie.rpc.Dictionary[buffer.ReadVarint()];
					var newValue = ibelie.rpc[compName]['Deserialize' + property](buffer.Bytes())[0];
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
					var args = entity['Deserialize' + name](data);
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
	var size = entity.ByteSize() + tyts.SizeVarint(method);
	if (data) {
		size += data.length;
	}
	var protobuf = new tyts.ProtoBuf(new Uint8Array(size));
	entity.SerializeUnsealed(protobuf);
	protobuf.WriteVarint(method);
	if (data) {
		protobuf.WriteBytes(data);
	}
	this.socket.send(protobuf.ToBase64());
};

ibelie.rpc.Connection.prototype.disconnect = function() {
	this.socket.close();
};
%s`, strings.Join(requires, ""), strings.Join(methods, ""))))

	ioutil.WriteFile(path.Join(dir, "rpc.js"), buffer.Bytes(), 0666)
}
