// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package uuid

import (
	"sort"
)

func Compare(a UUID, b UUID) (c int) {
	for i, aa := range a {
		c = int(aa) - int(b[i])
		if c != 0 {
			return
		}
	}
	return
}

type UUIDSlice []UUID

func (s UUIDSlice) Len() int           { return len(s) }
func (s UUIDSlice) Less(i, j int) bool { return Compare(s[i], s[j]) < 0 }
func (s UUIDSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s UUIDSlice) Sort()              { sort.Sort(s) }
