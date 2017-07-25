// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package strid

import (
	"github.com/ibelie/tygo"
	"io"
)

type ID string

func (s ID) ByteSize() (size int) {
	l := len([]byte(s))
	return tygo.SizeVarint(l) + l
}

func (s ID) Serialize(output *tygo.Protobuf) {
	output.WriteBuf([]byte(s))
}

func (s *ID) Deserialize(input *tygo.Protobuf) (err error) {
	if x, e := input.ReadBuf(); e == nil {
		*s = ID(x)
	} else {
		err = e
	}
}
