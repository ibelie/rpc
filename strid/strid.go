// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package strid

import (
	"github.com/ibelie/tygo"
	"math/rand"
	"time"
)

type ID string

const (
	ZERO ID = ""
	SIZE    = 10
)

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var src = rand.NewSource(time.Now().UnixNano())

func New() ID {
	b := make([]byte, SIZE)
	for i, cache, remain := SIZE-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return ID(b)
}

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
