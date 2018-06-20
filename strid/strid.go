// Copyright 2017-2018 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package strid

import (
	"crypto/md5"
	"github.com/ibelie/ruid"
	"github.com/ibelie/tygo"
)

type STRID string

const ZERO STRID = ""

func New() STRID {
	return STRID(ruid.New().String())
}

func (s STRID) Hash() ruid.ID {
	hash := md5.Sum([]byte(s))
	return STRID(hash[:])
}

func (s STRID) Lt(o ruid.ID) bool {
	return s < o.(STRID)
}

func (s STRID) Ge(o ruid.ID) bool {
	return s >= o.(STRID)
}

func (s STRID) Nonzero() bool {
	return s != ZERO
}

func (s STRID) String() string {
	return string(s)
}

func (s STRID) ByteSize() (size int) {
	return tygo.SizeBuffer([]byte(s))
}

func (s STRID) Serialize(output *tygo.ProtoBuf) {
	output.WriteBuf([]byte(s))
}

func (s *STRID) Deserialize(input *tygo.ProtoBuf) (err error) {
	x, err := input.ReadBuf()
	*s = STRID(x)
	return
}

type STRIdentity int

var STRIdent STRIdentity = 0

func (_ STRIdentity) New() ruid.ID {
	return STRID(ruid.New().String())
}

func (_ STRIdentity) Zero() ruid.ID {
	return ZERO
}

func (_ STRIdentity) Deserialize(input *tygo.ProtoBuf) (s ruid.ID, err error) {
	x, err := input.ReadBuf()
	s = STRID(x)
	return
}

func (_ STRIdentity) GetIDs(bytes []byte) (ids []ruid.ID) {
	for _, id := range ruid.RUIdent.GetIDs(bytes) {
		ids = append(ids, STRID(id.String()))
	}
	return
}
