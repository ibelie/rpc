#-*- coding: utf-8 -*-
# Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
# Use of this source code is governed by The MIT License
# that can be found in the LICENSE file.

def extract(modules):
	for module in modules:
		__import__(module)


if __name__ == '__main__':
	import argparse
	parser = argparse.ArgumentParser(description = 'ibelie.rpc.python.extract')
	parser.add_argument('-m', '--modules', nargs = '+', dest = "modules")
	extract(parser.parse_args().modules)
