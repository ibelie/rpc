// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package uuid

import (
	"sort"
)

type ID UUID

func Compare(a ID, b ID) (c int) {
	for i, aa := range a {
		c = int(aa) - int(b[i])
		if c != 0 {
			return
		}
	}
	return
}

type IDSlice []ID

func (s IDSlice) Len() int           { return len(s) }
func (s IDSlice) Less(i, j int) bool { return Compare(s[i], s[j]) < 0 }
func (s IDSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s IDSlice) Sort()              { sort.Sort(s) }
