// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package uuid

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"github.com/ibelie/ruid"
	"github.com/ibelie/tygo"
)

type ID UUID

var ZERO = ID{}

func New() ID {
	return ID(NewV1())
}

func (u ID) Hash() ruid.ID {
	return ID(md5.Sum(u[:]))
}

func (u ID) Lt(o ruid.ID) bool {
	i := o.(ID)
	return bytes.Compare(u[:], i[:]) < 0
}

func (u ID) Ge(o ruid.ID) bool {
	i := o.(ID)
	return bytes.Compare(u[:], i[:]) >= 0
}

func (u ID) Nonzero() bool {
	return u != ZERO
}

func (u ID) String() string {
	return base64.RawURLEncoding.EncodeToString(u[:])
}

func (u ID) ByteSize() (size int) {
	return 16
}

func (u ID) Serialize(output *tygo.ProtoBuf) {
	output.Write(u[:])
}

func (u *ID) Deserialize(input *tygo.ProtoBuf) (err error) {
	_, err = input.Read(u[:])
	return
}

type UUIdentity int

var UUIdent UUIdentity = 0

func (_ UUIdentity) New() ruid.ID {
	return ID(NewV1())
}

func (_ UUIdentity) Zero() ruid.ID {
	return ZERO
}

func (_ UUIdentity) Deserialize(input *tygo.ProtoBuf) (ruid.ID, error) {
	u := ID{}
	_, err := input.Read(u[:])
	return u, err
}

func (_ UUIdentity) GetIDs(bytes []byte) (ids []ruid.ID) {
	for i, n := 0, len(bytes)-16; i <= n; i += 16 {
		u := ID{}
		copy(u[:], bytes[i:])
		ids = append(ids, u)
	}
	return
}
