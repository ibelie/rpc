// Copyright 2017 - 2018 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package main

import (
	"flag"

	"github.com/ibelie/rpc"
	"github.com/ibelie/tygo"
)

func main() {
	pkg := flag.String("pkg", "", "target package")
	flag.Parse()
	rpc.Go(*pkg, tygo.Extract(*pkg, rpc.Inject))
}
