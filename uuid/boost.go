// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package uuid

import (
	"bytes"
	"sort"
)

type ID UUID

func Compare(a ID, b ID) int {
	return bytes.Compare(a[:], b[:])
}

func (u ID) ByteSize() (size int) {
	return 16
}

func (u ID) Serialize(writer io.Writer) {
	writer.Write(u[:])
}

func (u *ID) Deserialize(reader io.Reader) (err error) {
	_, err = reader.Read(u[:])
	return
}

type IDSlice []ID

func (s IDSlice) Len() int           { return len(s) }
func (s IDSlice) Less(i, j int) bool { return Compare(s[i], s[j]) < 0 }
func (s IDSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s IDSlice) Sort()              { sort.Sort(s) }
