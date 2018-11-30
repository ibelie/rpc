// Copyright 2017 - 2018 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

"use strict";
var ts = require("typescript");

function Extract(fileNames, options) {
	var program = ts.createProgram(fileNames, options);
	var checker = program.getTypeChecker();
	var pkg = {
		Files: fileNames,
		Objects: []
	};
	for (var _i = 0, _a = program.getSourceFiles(); _i < _a.length; _i++) {
		ts.forEachChild(_a[_i], visit);
	}

	console.info(JSON.stringify(pkg, undefined, 4));
	return;

	function visit(node, mod) {
		var exported = isNodeExported(node);
		if (exported && mod && ts.isClassDeclaration(node) && node.name) {
			pkg.Objects.push(processObject(node, mod));
		} else if (mod && ts.isInterfaceDeclaration(node) && node.name && node.heritageClauses) {
			pkg.Objects.push(processObject(node, mod));
		} else if (exported && ts.isModuleDeclaration(node)) {
			var m = checker.getSymbolAtLocation(node.name).getName();
			mod = mod ? mod + '.' + m : m;
			ts.forEachChild(node, (node) => { visit(node, mod); });
		} else if (ts.isModuleBlock(node)) {
			ts.forEachChild(node, (node) => { visit(node, mod); });
		}
	}

	function processObject(node, mod) {
		var symbol = checker.getSymbolAtLocation(node.name);
		var object = {
			Name: symbol.getName(),
			Module: mod,
			Parents: [],
			Fields:  [],
			Methods: []
		};
		if (node.heritageClauses) {
			for (var _i = 0, _a = node.heritageClauses; _i < _a.length; _i++) {
				for (var _j = 0, _b = _a[_i].types; _j < _b.length; _j++) {
					object.Parents.push({ Simple: _b[_j].getText() });
				}
			}
		}
		symbol.members.forEach((v, k, _) => {
			var t = checker.getTypeOfSymbolAtLocation(v, v.valueDeclaration);
			if (t.getCallSignatures() && t.getCallSignatures().length > 0) {
				var method = {
					Name: k,
					Params: t.getCallSignatures()[0].parameters.map(function (x) { return type(checker.getTypeOfSymbolAtLocation(x, x.valueDeclaration), x.valueDeclaration); }),
					Document: ts.displayPartsToString(v.getDocumentationComment(checker))
				};
				var result = type(t.getCallSignatures()[0].getReturnType());
				if (result.Simple != "void") {
					method.Result = result;
				}
				object.Methods.push(method);
			} else {
				object.Fields.push({
					Name: k,
					Type: type(t, v.valueDeclaration),
					Document: ts.displayPartsToString(v.getDocumentationComment(checker))
				});
			}
		});
		return object;
	}

	function isNodeExported(node) {
		return (ts.getCombinedModifierFlags(node) & ts.ModifierFlags.Export) !== 0 || (!!node.parent && node.parent.kind === ts.SyntaxKind.SourceFile);
	}

	function type(t, d) {
		var s = d ? d.type.getText() : checker.typeToString(t);
		if (t.types) {
			var types = [];
			for (var typ of t.types) {
				types.push(type(typ));
			}
			return { Variant: types };
		} else if (s == 'string') {
			return { Simple: s };
		} else if (s.substr(s.length - 2) == "[]") {
			return { List: type(checker.getIndexTypeOfType(t, ts.IndexKind.Number)) };
		} else if (checker.getIndexTypeOfType(t, ts.IndexKind.Number)) {
			return { Key: { Simple: "number" }, Value: type(checker.getIndexTypeOfType(t, ts.IndexKind.Number)) };
		} else if (checker.getIndexTypeOfType(t, ts.IndexKind.String)) {
			return { Key: { Simple: "string" }, Value: type(checker.getIndexTypeOfType(t, ts.IndexKind.String)) };
		} else {
			return { Simple: s };
		}
	}
}

Extract(process.argv.slice(2), {
	target: ts.ScriptTarget.ES5, module: ts.ModuleKind.CommonJS
});
