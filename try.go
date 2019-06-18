// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
)

var count int // global count of all `try` candidates

// tryFile identifies statements in f that are potential candidates for `try`,
// lists their positions (-l flag), or rewrites them in place using `try` (-r flag).
// tryFile reports whether the block was rewritten.
//
// Candiates satisfy the following requirements:
// - If the statement is an assignment, it is followed by an `if` statement:
//
//	v1, v2, ..., vn, err = f() // can also be :=
//	if err != nil {
//		return ..., err
//	}
//
// - If the statement is an `if` statement, it is of the form:
//
//	if v1, v2, ..., vn, err = f(); err != nil { // can also be :=
//		return ..., err
//
// In both cases, the `return` returns zero values followed by `err`.
// Local function closures are ignored at the moment.
func tryFile(f *ast.File) bool {
	dirty := false
	for _, d := range f.Decls {
		if f, ok := d.(*ast.FuncDecl); ok {
			if hasErrorResult(f.Type) && f.Body != nil {
				if tryBlock(f.Body) {
					dirty = true
				}
			}
		}
	}
	return dirty
}

// tryBlock is like tryFile but operates on a block b.
func tryBlock(b *ast.BlockStmt) bool {
	dirty := false
	var p ast.Stmt // previous statement
	for i, s := range b.List {
		switch s := s.(type) {
		case *ast.BlockStmt:
			if tryBlock(s) {
				dirty = true
			}
		case *ast.ForStmt:
			if tryBlock(s.Body) {
				dirty = true
			}
		case *ast.RangeStmt:
			if tryBlock(s.Body) {
				dirty = true
			}
		case *ast.SelectStmt:
			if tryBlock(s.Body) {
				dirty = true
			}
		case *ast.SwitchStmt:
			if tryBlock(s.Body) {
				dirty = true
			}
		case *ast.TypeSwitchStmt:
			if tryBlock(s.Body) {
				dirty = true
			}
		case *ast.IfStmt:
			if isErrTest(s.Cond) && isErrReturn(s.Body) {
				if s.Init == nil && isErrAssign(p) {
					count++
					if *list {
						listPos(count, p)
					}
					if *rewrite {
						b.List[i-1] = rewriteAssign(p, s.End())
						b.List[i] = nil // remove `if`
						dirty = true
					}
				} else if isErrAssign(s.Init) {
					count++
					if *list {
						listPos(count, s)
					}
					if *rewrite {
						b.List[i] = rewriteAssign(s.Init, s.End())
						dirty = true
					}
				}
			}
		}
		p = s
	}

	if dirty {
		i := 0
		for _, s := range b.List {
			if s != nil {
				b.List[i] = s
				i++
			}
		}
		b.List = b.List[:i]
	}

	return dirty
}

// listPos prints the position of statement s, numbered by n.
func listPos(n int, s ast.Stmt) {
	pos := fset.Position(s.Pos())
	fmt.Printf("%5d  %s:%d\n", n, pos.Filename, pos.Line)
}

// rewriteAssign assumes that s is an assignment that is a potential candidate
// for `try` and rewrites it accordingly. It returns the new assignment (or the
// assignment's rhs if there's no lhs anymore).
func rewriteAssign(s ast.Stmt, end token.Pos) ast.Stmt {
	a := s.(*ast.AssignStmt)
	lhs := a.Lhs[:len(a.Lhs)-1]
	rhs := a.Rhs[0]
	pos := rhs.Pos()
	rhs0 := &ast.CallExpr{Fun: &ast.Ident{NamePos: pos, Name: "try"}, Lparen: pos, Args: []ast.Expr{a.Rhs[0]}, Rparen: end}
	if isBlanks(lhs) {
		// no lhs anymore - no need for assignment
		return &ast.ExprStmt{X: rhs0}
	}
	a.Lhs = lhs
	a.Rhs[0] = rhs0
	return a
}

// isBlanks reports whether list is empty or contains only blank identifiers.
func isBlanks(list []ast.Expr) bool {
	for _, x := range list {
		if x, ok := x.(*ast.Ident); !ok || x.Name != "_" {
			return false
		}
	}
	return true
}

// asErrAssign reports whether s is an assignment statement of the form:
//
//      v1, v2, ... vn, err  = f()
//      v1, v2, ... vn, err := f()
//
// where the vi are arbitrary expressions or variables (n may also be 0),
// err is the identifier "err", and f() stands for a function call.
func isErrAssign(s ast.Stmt) bool {
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok != token.ASSIGN && a.Tok != token.DEFINE {
		return false
	}
	return len(a.Lhs) > 0 && isName(a.Lhs[len(a.Lhs)-1], "err") && len(a.Rhs) == 1 && isCall(a.Rhs[0])
}

// isCall reports whether x is a (function) call.
// (A conversion may appear as a false positive).
func isCall(x ast.Expr) bool {
	_, ok := x.(*ast.CallExpr)
	return ok
}

// isErrTest reports whether x is a the binary operation "err != nil".
func isErrTest(x ast.Expr) bool {
	c, ok := x.(*ast.BinaryExpr)
	return ok && c.Op == token.NEQ && isName(c.X, "err") && isName(c.Y, "nil")
}

// isErrReturn reports whether b contains a single return statement
// that is either empty, or reports all zero values followed by a final
// value called "err".
func isErrReturn(b *ast.BlockStmt) bool {
	if len(b.List) != 1 {
		return false
	}
	ret, ok := b.List[0].(*ast.ReturnStmt)
	if !ok {
		return false
	}
	for i, x := range ret.Results {
		if i < len(ret.Results)-1 {
			if !isZero(x) {
				return false
			}
		} else {
			if !isName(x, "err") {
				return false
			}
		}
	}
	return true
}

// hasErrorResult reports whether sig has a final result with type name "error".
func hasErrorResult(sig *ast.FuncType) bool {
	res := sig.Results
	if res == nil || len(res.List) == 0 {
		return false // no results
	}
	last := res.List[len(res.List)-1].Type
	return isName(last, "error")
}

// isZero reports whether x is a zero value.
func isZero(x ast.Expr) bool {
	switch x := x.(type) {
	case *ast.Ident:
		return x.Name == "nil"
	case *ast.BasicLit:
		v := x.Value
		if len(v) == 0 {
			return false
		}
		switch x.Kind {
		case token.INT:
			z, err := strconv.ParseInt(v, 0, 64)
			return err == nil && z == 0
		case token.FLOAT:
			z, err := strconv.ParseFloat(v, 64)
			return err == nil && z == 0
		case token.IMAG:
			z, err := strconv.ParseFloat(v[:len(v)-1], 64)
			return err == nil && z == 0
		case token.CHAR:
			return v == "0" // there are more cases here
		case token.STRING:
			return v == `""` || v == "``"
		}
	case *ast.CompositeLit:
		return len(x.Elts) == 0
	}
	return false
}

// isName reports whether x is an identifier with the given name
func isName(x ast.Expr, name string) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Name == name
}
