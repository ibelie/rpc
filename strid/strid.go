// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package strid

import (
	"github.com/ibelie/tygo"
)

type ID string

const ZERO ID = ""

func (s ID) String() string {
	return string(s)
}

func (s ID) ByteSize() (size int) {
	l := len([]byte(s))
	return tygo.SizeVarint(uint64(l)) + l
}

func (s ID) Serialize(output *tygo.ProtoBuf) {
	output.WriteBuf([]byte(s))
}

func (s *ID) Deserialize(input *tygo.ProtoBuf) (err error) {
	if x, e := input.ReadBuf(); e == nil {
		*s = ID(x)
	} else {
		err = e
	}
	return
}
