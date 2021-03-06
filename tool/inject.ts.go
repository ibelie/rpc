// Copyright 2017 - 2018 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package main

import (
	"flag"

	"github.com/ibelie/rpc"
)

func main() {
	ident := flag.String("id", "", "identifier type")
	tsOut := flag.String("ts", "", "output typescript dir")
	goOut := flag.String("go", "", "output golang dir")
	flag.Parse()
	rpc.Typescript(*ident, *tsOut, *goOut, flag.Args())
}
