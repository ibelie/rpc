// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/ibelie/rpc/strid"
	"github.com/ibelie/rpc/uuid"
	"github.com/ibelie/ruid"
	"github.com/ibelie/tygo"
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
	goog.require('ibelie.tyts.String');
`,
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

func injectJavascript(identName string, dir string, entities []*Entity, behaviors []*Behavior) {
	ident := IDENT_FromString(identName)
	ioutil.WriteFile(path.Join(dir, "entity.js"), []byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

goog.provide('ibelie.rpc.Entity');
goog.provide('ibelie.rpc.ZERO_ID');
%s
ibelie.rpc.ZERO_ID = %q;

ibelie.rpc.Entity = function() {
	this.__class__ = 'Entity';
	this.isAwake = false;
	this.ID = ibelie.rpc.ZERO_ID;
	this.Key = ibelie.rpc.ZERO_ID;
	this.Type = '';
};
`, JSID_REQUIRE[ident], JSID_ZERO[ident])), 0666)

	var buffer bytes.Buffer
	requireMap := make(map[string]bool)
	methodsMap := make(map[string]bool)

	var methods []string
	var entcodes []string
	for _, e := range entities {
		var components []string
		for _, c := range e.Components {
			if c.Protocol == nil {
				continue
			}

			compModule := tygo.JS_MODULE
			if compModule == "" {
				compModule = c.Protocol.Package
			}
			compModule = strings.Replace(compModule, "/", ".", -1)
			requireMap[fmt.Sprintf(`
goog.require('%s.%s');`, compModule, c.Name)] = true
			components = append(components, fmt.Sprintf(`
			_this['%s'] = new %s.%s();`, c.Name, compModule, c.Name))

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
				localParams := append([]string{"this"}, params...)
				methods = append(methods, fmt.Sprintf(`
ibelie.rpc.Entity.prototype['D_%s'] = function(data) {
	return %s.%s['D_%sParam'](data);
};

ibelie.rpc.Entity.prototype.%s = function(%s) {
	if (!this.isAwake) {
		console.warn('[Entity] Not awake:', this);
		return;
	}
	for (var b of this.Behaviors) {
		var m = b['%s'];
		m && m(%s);
	}
	var data = %s.%s['S_%sParam'](%s);
	this.connection.send(this, this.connection.SymDict['%s'], data);
};
`, m.Name, compModule, c.Name, m.Name, m.Name, strings.Join(params, ", "),
					m.Name, strings.Join(localParams, ", "),
					compModule, c.Name, m.Name, strings.Join(params, ", "), m.Name))
				methodsMap[m.Name] = true
			}
		}

		var entBehaviors []string
		for _, b := range e.Behaviors {
			behaviorModule := b.Name
			if b.Module != "" {
				behaviorModule = b.Module + "." + b.Name
			}

			for _, m := range b.Methods {
				if ok, exist := methodsMap[m]; exist && ok {
					continue
				}
				methods = append(methods, fmt.Sprintf(`
ibelie.rpc.Entity.prototype.%s = function() {
	if (!this.isAwake) {
		console.warn('[Entity] Not awake:', this);
		return;
	}
	var args = Array.prototype.concat.apply([this], arguments);
	for (var b of this.Behaviors) {
		var m = b['%s'];
		m && m.apply(args);
	}
};
`, m, m))
				methodsMap[m] = true
			}

			requireMap[fmt.Sprintf(`
goog.require('%s');`, behaviorModule)] = true
			entBehaviors = append(entBehaviors, fmt.Sprintf(`
			%s,`, behaviorModule))
		}
		sort.Strings(entBehaviors)

		entcodes = append(entcodes, fmt.Sprintf(`
	'%s': (function (_super) {
		__extends(%s, _super);
		function %s() {
			var _this = _super.call(this) || this;%s
			return _this;
		}
		%s.prototype.Behaviors = [%s
		];
		__reflect(%s.prototype, '%s');
		return %s;
	}(ibelie.rpc.Entity)),`, e.Name, e.Name, e.Name, strings.Join(components, ""),
			e.Name, strings.Join(entBehaviors, ""), e.Name, e.Name, e.Name))
	}

	for _, b := range behaviors {
		log.Println("behavior", b.Name, b.Entities)
	}

	var requires []string
	for require, ok := range requireMap {
		if ok {
			requires = append(requires, require)
		}
	}
	sort.Strings(requires)

	buffer.Write([]byte(fmt.Sprintf(`// Generated by ibelie-rpc.  DO NOT EDIT!

goog.provide('ibelie.rpc.entities');
goog.provide('ibelie.rpc.behaviors');
goog.provide('ibelie.rpc.Connection');

goog.require('ibelie.rpc.Entity');
goog.require('ibelie.rpc.ZERO_ID');

goog.require('ibelie.tyts.ProtoBuf');
goog.require('ibelie.tyts.SizeVarint');
goog.require('ibelie.tyts.SymbolEncodedLen');%s

var __reflect = (this && this.__reflect) || function (p, c, t) {
	p.__class__ = c, t ? t.push(c) : t = [c], p.__types__ = p.__types__ ? t.concat(p.__types__) : t;
};

var __extends = (this && this.__extends) || function (d, b) {
	for (var p in b) if (b.hasOwnProperty(p)) d[p] = b[p];
	function __() { this.constructor = d; }
	d.prototype = b === null ? Object.create(b) : (__.prototype = b.prototype, new __());
};

ibelie.rpc.Entity.prototype.ByteSize = function() {
	var size = 1 + ibelie.tyts.SymbolEncodedLen(this.Type);
	if (this.ID != ibelie.rpc.ZERO_ID) {
		size += %s;
	}
	if (this.Key != ibelie.rpc.ZERO_ID) {
		size += %s;
	}
	return size;
};

ibelie.rpc.Entity.prototype.Serialize = function() {
	var protobuf = new ibelie.tyts.ProtoBuf(new Uint8Array(this.ByteSize()));
	var t = 0;
	if (this.ID != ibelie.rpc.ZERO_ID) {
		t |= 1;
	}
	if (this.Key != ibelie.rpc.ZERO_ID) {
		t |= 2;
	}
	protobuf.WriteByte(t);
	if (this.ID != ibelie.rpc.ZERO_ID) {
		%s;
	}
	if (this.Key != ibelie.rpc.ZERO_ID) {
		%s;
	}
	protobuf.EncodeSymbol(this.Type);
	return protobuf.buffer;
};

ibelie.rpc.Entity.prototype.Deserialize = function(data) {
	var protobuf = new ibelie.tyts.ProtoBuf(data);
	var t = protobuf.ReadByte();
	this.ID = (t & 1) ? %s : ibelie.rpc.ZERO_ID;
	this.Key = (t & 2) ? %s : ibelie.rpc.ZERO_ID;
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
	conn.send(e, conn.SymDict['OBSERVE']);
	conn.entities[entity.ID] = entity;
	return entity;
};

ibelie.rpc.Entity.prototype.Drop = function(e) {
	if (!e || !e.isAwake) {
		console.warn('[Entity] Not awaked:', e);
		return;
	}
	e.onDrop && e.onDrop();
	e.isAwake = false;
	var conn = this.connection;
	conn.send(e, conn.SymDict['IGNORE']);
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
				if (entity[name] && entity[name].Deserialize) {
					entity[name].Deserialize(data);
				} else if (!entity.isAwake) {
					console.error('[Connection] Entity is not awake:', id, name, entity);
					continue;
				} else if (name == 'NOTIFY') {
					var buffer = new ibelie.tyts.ProtoBuf(data);
					var compName = conn.Symbols[buffer.ReadVarint()];
					var property = conn.Symbols[buffer.ReadVarint()];
					var component = entity[compName];
					var newValue = component['D_' + property](buffer.Bytes())[0];
					var oldValue = component[property];
					var args = [component, oldValue, newValue];
					if (oldValue.concat) {
						component[property] = oldValue.concat(newValue);
					} else if ((newValue instanceof Object) && !newValue.__class__) {
						if (!component[property]) {
							component[property] = {};
						}
						for (var k in newValue) {
							var o = oldValue[k];
							var n = newValue[k];
							oldValue[k] = n;
							args = [component, k, o, n];
						}
					} else {
						component[property] = newValue;
					}
					for (var b of entity.Behaviors) {
						var c = b[compName];
						var h = c && c[property];
						h && h.apply(args);
					}
				} else {
					var args = entity['D_' + name](data);
					for (var b of entity.Behaviors) {
						var m = b[name];
						m && m.apply(args);
					}
				}
			}
			if (entity && !entity.isAwake) {
				entity.isAwake = true;
				entity.onAwake && entity.onAwake();
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
	if (entity.ID != ibelie.rpc.ZERO_ID) {
		size += %s;
		t |= 1;
	}
	if (entity.Key != ibelie.rpc.ZERO_ID) {
		size += %s;
		t |= 2;
	}
	if (data) {
		size += data.length;
	}
	var protobuf = new ibelie.tyts.ProtoBuf(new Uint8Array(size));
	protobuf.WriteVarint(t);
	if (entity.ID != ibelie.rpc.ZERO_ID) {
		%s;
	}
	if (entity.Key != ibelie.rpc.ZERO_ID) {
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
%s
ibelie.rpc.entities = {%s
};
`, strings.Join(requires, ""),
		JSID_BYTESIZE[ident]("this.ID"), JSID_BYTESIZE[ident]("this.Key"),
		JSID_WRITE[ident]("protobuf", "this.ID"), JSID_WRITE[ident]("protobuf", "this.Key"),
		JSID_READ[ident]("protobuf"), JSID_READ[ident]("protobuf"),
		JSID_READ[ident]("protobuf"), JSID_READ[ident]("protobuf"),
		JSID_BYTESIZE[ident]("this.ID"), JSID_BYTESIZE[ident]("this.Key"),
		JSID_WRITE[ident]("protobuf", "entity.ID"), JSID_WRITE[ident]("protobuf", "entity.Key"),
		strings.Join(methods, ""), strings.Join(entcodes, ""))))

	ioutil.WriteFile(path.Join(dir, "rpc.js"), buffer.Bytes(), 0666)
}
