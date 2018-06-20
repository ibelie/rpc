// Copyright 2017 - 2018 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package strid

import (
	"fmt"
	"github.com/ibelie/ruid"
	"sort"
	"testing"
)

func randomRing(ring *ruid.Ring, n int) {
	count := make(map[string]int)
	for i := 0; i < n; i++ {
		id := STRIdent.New()
		node, _ := ring.Get(id)
		if _, exist := count[node]; !exist {
			count[node] = 0
		}
		count[node]++
	}
	var sorted []int
	for _, c := range count {
		sorted = append(sorted, c)
	}
	sort.Ints(sorted)
	for i, c := range sorted {
		fmt.Printf("%d, %d,\n", i, c)
	}
}

func TestRing(t *testing.T) {
	var nodes []string
	weights := make(map[string]int)
	for i := 0; i < 100; i++ {
		node := fmt.Sprintf("node_%d", i)
		nodes = append(nodes, node)
		weights[node] = i
	}
	randomRing(ruid.WeightedRing(STRIdent, weights), 100000)
	randomRing(ruid.NewRing(STRIdent, nodes...), 10000)
}
