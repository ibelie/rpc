// Copyright 2017 ibelie, Chen Jie, Joungtao. All rights reserved.
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

"use strict";
var ts = require("typescript");

function Decorate(fileNames, options) {
    var program = ts.createProgram(fileNames, options);
    var checker = program.getTypeChecker();
    var pkg = {
        Name: "client",
        Path: "typescript",
        Imports: [],
        Objects: {}
    };
    for (var _i = 0, _a = program.getSourceFiles(); _i < _a.length; _i++) {
        var sourceFile = _a[_i];
        ts.forEachChild(sourceFile, visit);
    }
    console.info(JSON.stringify(pkg, undefined, 4));
    return;
    function visit(node) {
        var exported = isNodeExported(node);
        if (exported && node.kind === ts.SyntaxKind.ClassDeclaration) {
            var object = processObject(node);
            pkg.Objects[object.Name] = object;
        }
        else if (exported && node.kind === ts.SyntaxKind.ModuleDeclaration) {
            ts.forEachChild(node, visit);
        }
        else if (node.kind === ts.SyntaxKind.ModuleBlock) {
            ts.forEachChild(node, visit);
        }
    }
    function processObject(node) {
        var symbol = checker.getSymbolAtLocation(node.name);
        var object = {
            Name: symbol.getName(),
            Parents: {},
            Fields: {},
            Methods: {}
        };
        if (node.heritageClauses) {
            for (var _i = 0, _a = node.heritageClauses; _i < _a.length; _i++) {
                var h = _a[_i];
                var p = h.getLastToken().getText();
                object.Parents[p] = { Type: { Simple: p }, Decorators: {} };
            }
        }
        for (var m in symbol.members) {
            var s = symbol.members[m];
            var t = checker.getTypeOfSymbolAtLocation(s, s.valueDeclaration);
            if (t.getCallSignatures() && t.getCallSignatures().length > 0) {
                object.Methods[m] = {
                    Name: m,
                    Result: [],
                    Params: t.getCallSignatures()[0].parameters.map(function (x) { return type(checker.getTypeOfSymbolAtLocation(x, x.valueDeclaration)); }),
                    Decorators: decorate(ts.displayPartsToString(s.getDocumentationComment()))
                };
                var result = type(t.getCallSignatures()[0].getReturnType());
                if (result.Simple != "void") {
                    object.Methods[m].Result = [result];
                }
            }
            else {
                object.Fields[m] = {
                    Type: type(t),
                    Name: m,
                    Decorators: decorate(ts.displayPartsToString(s.getDocumentationComment()))
                };
            }
        }
        return object;
    }
    function isNodeExported(node) {
        return (node.flags & ts.NodeFlags.Export) !== 0 || (node.parent && node.parent.kind === ts.SyntaxKind.SourceFile);
    }
    function type(t) {
        if (t.flags & ts.TypeFlags.StringLike) {
            return { Simple: checker.typeToString(t) };
        }
        else if (checker.typeToString(t).substr(checker.typeToString(t).length - 2) == "[]") {
            return { List: type(checker.getIndexTypeOfType(t, ts.IndexKind.Number)) };
        }
        else if (checker.getIndexTypeOfType(t, ts.IndexKind.Number)) {
            return { Key: { Simple: "number" }, Value: type(checker.getIndexTypeOfType(t, ts.IndexKind.Number)) };
        }
        else if (checker.getIndexTypeOfType(t, ts.IndexKind.String)) {
            return { Key: { Simple: "string" }, Value: type(checker.getIndexTypeOfType(t, ts.IndexKind.String)) };
        }
        else {
            return { Simple: checker.typeToString(t) };
        }
    }
    function decorate(text) {
        var decorators = {};
        var re = /@(\w+)(?:\((.+?)\))?/g;
        for (var match = re.exec(text); match; match = re.exec(text)) {
            decorators[match[1]] = { Name: match[1], Params: match[2] ? match[2].split(",").map(function (x) { return x.trim(); }) : [] };
        }
        return decorators;
    }
}
Decorate(process.argv.slice(2), {
    target: ts.ScriptTarget.ES5, module: ts.ModuleKind.CommonJS
});
