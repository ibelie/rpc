// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
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

	function visit(node) {
		var exported = isNodeExported(node);
		if (exported && ts.isClassDeclaration(node) && node.name) {
			pkg.Objects.push(processObject(node));
		} else if (exported && ts.isModuleDeclaration(node)) {
			ts.forEachChild(node, visit);
		} else if (ts.isModuleBlock(node)) {
			ts.forEachChild(node, visit);
		}
	}

	function processObject(node) {
		var symbol = checker.getSymbolAtLocation(node.name);
		var object = {
			Name: symbol.getName(),
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
		for (var m in symbol.members) {
			var s = symbol.members[m];
			var t = checker.getTypeOfSymbolAtLocation(s, s.valueDeclaration);
			if (t.getCallSignatures() && t.getCallSignatures().length > 0) {
				var method = {
					Name: m,
					Params: t.getCallSignatures()[0].parameters.map(function (x) { return type(checker.getTypeOfSymbolAtLocation(x, x.valueDeclaration), x.valueDeclaration); }),
					Document: ts.displayPartsToString(s.getDocumentationComment())
				};
				var result = type(t.getCallSignatures()[0].getReturnType());
				if (result.Simple != "void") {
					method.Result = result;
				}
				object.Methods.push(method);
			} else {
				object.Fields.push({
					Name: m,
					Type: type(t, s.valueDeclaration),
					Document: ts.displayPartsToString(s.getDocumentationComment())
				});
			}
		}
		return object;
	}

	function isNodeExported(node) {
		return (ts.getCombinedModifierFlags(node) & ts.ModifierFlags.Export) !== 0 || (!!node.parent && node.parent.kind === ts.SyntaxKind.SourceFile);
	}

	function type(t, d) {
		if (t.flags & ts.TypeFlags.StringLike) {
			return { Simple: checker.typeToString(t, d) };
		} else if (checker.typeToString(t).substr(checker.typeToString(t).length - 2) == "[]") {
			return { List: type(checker.getIndexTypeOfType(t, ts.IndexKind.Number)) };
		} else if (checker.getIndexTypeOfType(t, ts.IndexKind.Number)) {
			return { Key: { Simple: "number" }, Value: type(checker.getIndexTypeOfType(t, ts.IndexKind.Number)) };
		} else if (checker.getIndexTypeOfType(t, ts.IndexKind.String)) {
			return { Key: { Simple: "string" }, Value: type(checker.getIndexTypeOfType(t, ts.IndexKind.String)) };
		} else {
			return { Simple: checker.typeToString(t, d) };
		}
	}
}

Extract(process.argv.slice(2), {
	target: ts.ScriptTarget.ES5, module: ts.ModuleKind.CommonJS
});
