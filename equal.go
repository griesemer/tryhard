// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "go/ast"

// TODO(gri) A more general version of this code should
// probably be in go/ast at some point.

// equal reports whether x and y are equal;
// i.e., whether they describe the same Go expression.
// equal is conservative and report false for cases
// that it ignores.
func equal(x, y ast.Expr) bool {
	// We cannot use reflect.DeepEqual because the go/ast also encodes position
	// and comment information which we need to ignore.
	// We could use printing and compare the strings but the code here is simple
	// and straight-forward; and this is much faster than printing.
	// (switch body framework generated with: https://play.golang.org/p/zMt7U03s0ys)
	switch x := x.(type) {
	case nil:
		if y == nil {
			return true
		}
	case *ast.Ident:
		if y, ok := y.(*ast.Ident); ok {
			return x.Name == y.Name
		}
	case *ast.Ellipsis:
		if y, ok := y.(*ast.Ellipsis); ok {
			return equal(x.Elt, y.Elt)
		}
	case *ast.BasicLit:
		if y, ok := y.(*ast.BasicLit); ok {
			return x.Kind == y.Kind && x.Value == y.Value // this is overly conservative
		}
	case *ast.FuncLit:
		// ignored for now (we don't have the code for comparing statements)
	case *ast.CompositeLit:
		if y, ok := y.(*ast.CompositeLit); ok {
			return !x.Incomplete && !y.Incomplete && equal(x.Type, y.Type) && equalLists(x.Elts, y.Elts)
		}
	case *ast.ParenExpr:
		if y, ok := y.(*ast.ParenExpr); ok {
			return equal(x.X, y.X)
		}
	case *ast.SelectorExpr:
		if y, ok := y.(*ast.SelectorExpr); ok {
			return equal(x.X, y.X) && equal(x.Sel, y.Sel)
		}
	case *ast.IndexExpr:
		if y, ok := y.(*ast.IndexExpr); ok {
			return equal(x.X, y.X) && equal(x.Index, y.Index)
		}
	case *ast.SliceExpr:
		if y, ok := y.(*ast.SliceExpr); ok {
			return equal(x.X, y.X) &&
				equal(x.Low, y.Low) && equal(x.High, y.High) && equal(x.Max, y.Max) &&
				x.Slice3 == y.Slice3
		}
	case *ast.TypeAssertExpr:
		if y, ok := y.(*ast.TypeAssertExpr); ok {
			return equal(x.X, y.X) && equal(x.Type, y.Type)
		}
	case *ast.CallExpr:
		if y, ok := y.(*ast.CallExpr); ok {
			return equal(x.Fun, y.Fun) && equalLists(x.Args, y.Args) && x.Ellipsis == y.Ellipsis
		}
	case *ast.StarExpr:
		if y, ok := y.(*ast.StarExpr); ok {
			return equal(x.X, y.X)
		}
	case *ast.UnaryExpr:
		if y, ok := y.(*ast.UnaryExpr); ok {
			return x.Op == y.Op && equal(x.X, y.X)
		}
	case *ast.BinaryExpr:
		if y, ok := y.(*ast.BinaryExpr); ok {
			return equal(x.X, y.X) && x.Op == y.Op && equal(x.Y, y.Y)
		}
	case *ast.KeyValueExpr:
		if y, ok := y.(*ast.KeyValueExpr); ok {
			return equal(x.Key, y.Key) && equal(x.Value, y.Value)
		}
	case *ast.ArrayType:
		if y, ok := y.(*ast.ArrayType); ok {
			return equal(x.Len, y.Len) && equal(x.Elt, y.Elt)
		}
	case *ast.StructType:
		if y, ok := y.(*ast.StructType); ok {
			return !x.Incomplete && !y.Incomplete && equalFields(x.Fields, y.Fields)
		}
	case *ast.FuncType:
		if y, ok := y.(*ast.FuncType); ok {
			return equalFields(x.Params, y.Params) && equalFields(x.Results, y.Results)
		}
	case *ast.InterfaceType:
		if y, ok := y.(*ast.InterfaceType); ok {
			return !x.Incomplete && !y.Incomplete && equalFields(x.Methods, y.Methods)
		}
	case *ast.MapType:
		if y, ok := y.(*ast.MapType); ok {
			return equal(x.Key, y.Key) && equal(x.Value, y.Value)
		}
	case *ast.ChanType:
		if y, ok := y.(*ast.ChanType); ok {
			return x.Dir == y.Dir && equal(x.Value, y.Value)
		}
	}
	return false
}

func equalLists(x, y []ast.Expr) bool {
	if len(x) != len(y) {
		return false
	}
	for i, x := range x {
		y := y[i]
		if !equal(x, y) {
			return false
		}
	}
	return true
}

func equalFields(x, y *ast.FieldList) bool {
	if x == nil || y == nil {
		return x == y
	}
	if len(x.List) != len(y.List) {
		return false
	}
	for i, x := range x.List {
		y := y.List[i]
		if !equalIdents(x.Names, y.Names) || !equal(x.Type, y.Type) || !equal(x.Tag, y.Tag) {
			return false
		}
	}
	return true
}

func equalIdents(x, y []*ast.Ident) bool {
	if len(x) != len(y) {
		return false
	}
	for i, x := range x {
		y := y[i]
		if !equal(x, y) {
			return false
		}
	}
	return true
}
