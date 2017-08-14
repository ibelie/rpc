#-*- coding: utf-8 -*-
# Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
# Use of this source code is governed by The MIT License
# that can be found in the LICENSE file.

def extract(files):
	import os
	import sys
	import codecs

	ignore = []

	def _scanScripts(sub):
		for i in os.listdir('%s/%s' % (path, sub)):
			sys.stdout.write('.')
			sys.stdout.flush()
			p = '%s/%s' % (sub, i) if sub else i
			fp = '%s/%s' % (path, p)
			if p in ignore or p[:-1] in ignore:
				continue
			elif os.path.isdir(fp):
				_scanScripts(p)
			elif fp in proto.timestamps and str(os.stat(fp).st_mtime) == proto.timestamps[fp]:
				continue
			elif i.endswith(('.py', '.pyc')):
				n = i.rpartition('.')[0]
				if not n.replace('_', '').isalnum() or n == '__init__' or (i.endswith('.pyc') and os.path.isfile(fp[:-1])) or \
					(sub and not os.path.isfile('%s/%s/__init__.py' % (path, sub)) and not os.path.isfile('%s/%s/__init__.pyc' % (path, sub))):
					continue
				proto.timestamps[fp] = str(os.stat(fp).st_mtime)
				m = n if not sub else '%s.%s' % (sub.replace('/', '.'), n)
				print '\n[Typy] Incremental proto:', m
				__import__(m)
				imported.add(m)
				proto.hasIncrement = True

	for path in files:
		if os.path.isfile(path) and path.endswith('.py'):
			with codecs.open(path, 'r', 'utf-8') as f:
				exec str(f.read()) in {}
		elif os.path.isdir(path):
			_scanScripts('')

	from microserver.classes import MetaEntity, MetaComponent
	print MetaComponent.Components
	for n, e in MetaEntity.Entities.iteritems():
		print n, e.____components__


if __name__ == '__main__':
	import sys
	extract(sys.argv[1:])
