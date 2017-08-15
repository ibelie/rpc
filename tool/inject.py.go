// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package main

import (
	"flag"
	"github.com/ibelie/rpc"
)

func main() {
	ident := flag.String("id", "", "identifier type")
	input := flag.String("in", "", "input python sources root")
	pyOut := flag.String("py", "", "output python dir")
	goOut := flag.String("go", "", "output golang dir")
	ignore := flag.String("ig", "", "ignore python modules")
	flag.Parse()
	entities := rpc.Python(*ident, *input, *pyOut, append(flag.Args(), *ignore))
	rpc.Proxy(*ident, *goOut, entities)
}
