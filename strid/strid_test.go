// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package strid

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

func randomRing(ring *Ring, n int) {
	count := make(map[string]int)
	for i := 0; i < n; i++ {
		buf := make([]byte, rand.Int()%10+10)
		rand.Read(buf)
		node, _ := ring.Get(STRID(buf))
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
	randomRing(WeightedRing(weights), 100000)
	randomRing(NewRing(nodes...), 10000)
}
