// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/ast"
	"go/token"
	"strconv"
)

// tryFile identifies statements in f that are potential candidates for `try`,
// lists their positions (-l flag), or rewrites them in place using `try` (-r flag)
// and sets *modified to true.
func tryFile(f *ast.File, modified *bool) {
	for _, d := range f.Decls {
		if f, ok := d.(*ast.FuncDecl); ok {
			count(Func, nil)
			if hasErrorResult(f.Type) && f.Body != nil {
				count(FuncError, nil)
				tryBlock(f.Body, modified)
			}
		}
	}
}

// tryBlock is like tryFile but operates on a block b.
func tryBlock(b *ast.BlockStmt, modified *bool) {
	dirty := false // if set, b.List contains nil entries
	var p ast.Stmt // previous statement
	for i, s := range b.List {
		count(Stmt, nil)
		switch s := s.(type) {
		case *ast.BlockStmt:
			tryBlock(s, modified)
		case *ast.ForStmt:
			tryBlock(s.Body, modified)
		case *ast.RangeStmt:
			tryBlock(s.Body, modified)
		case *ast.SelectStmt:
			tryBlock(s.Body, modified)
		case *ast.SwitchStmt:
			tryBlock(s.Body, modified)
		case *ast.TypeSwitchStmt:
			tryBlock(s.Body, modified)
		case *ast.IfStmt:
			count(If, nil)
			tryBlock(s.Body, modified)
			if s, ok := s.Else.(*ast.BlockStmt); ok {
				tryBlock(s, modified)
			}

			// condition must be of the form: <errname> != nil
			errname := *varname
			if !isErrTest(s.Cond, &errname) {
				break
			}
			count(IfErr, nil)

			// then block must be of the form: return ..., last (or just: return)
			ok, last := isReturn(s.Body)
			if !ok {
				count(ComplexBlock, s.Body)
				break
			}

			// last must be <err>
			if last != nil && !isName(last, errname) {
				count(ReturnExpr, s.Body)
				break
			}
			count(ReturnErr, nil)

			// else block must be absent
			if s.Else != nil {
				count(HasElse, s.Else)
				break
			}

			if s.Init == nil && isErrAssign(p, errname) {
				count(TryCand, s)
				if errname != "err" {
					count(NonErrName, s.Cond)
				}
				if *rewrite {
					b.List[i-1] = rewriteAssign(p, s.End())
					b.List[i] = nil // remove `if`
					dirty = true
					*modified = true
				}
			} else if isErrAssign(s.Init, errname) {
				count(TryCand, s)
				if errname != "err" {
					count(NonErrName, s.Cond)
				}
				if *rewrite {
					b.List[i] = rewriteAssign(s.Init, s.End())
					*modified = true
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
//      v1, v2, ... vn, <err>  = f()
//      v1, v2, ... vn, <err> := f()
//
// where the vi are arbitrary expressions or variables (n may also be 0),
// <err> is the variable errname, and f() stands for a function call.
func isErrAssign(s ast.Stmt, errname string) bool {
	a, ok := s.(*ast.AssignStmt)
	if !ok || a.Tok != token.ASSIGN && a.Tok != token.DEFINE {
		return false
	}
	return len(a.Lhs) > 0 && isName(a.Lhs[len(a.Lhs)-1], errname) && len(a.Rhs) == 1 && isCall(a.Rhs[0])
}

// isCall reports whether x is a (function) call.
// (A conversion may appear as a false positive).
func isCall(x ast.Expr) bool {
	_, ok := x.(*ast.CallExpr)
	return ok
}

// isErrTest reports whether x is a the binary operation "<err> != nil",
// where <err> stands for the name of the error variable. If *errname is
// the empty string, <err> may have any name, and *errname is set to it.
// Otherwise, <err> must be *errname.
func isErrTest(x ast.Expr, errname *string) bool {
	c, ok := x.(*ast.BinaryExpr)
	if ok && c.Op == token.NEQ && isName(c.Y, "nil") {
		if errv, ok := c.X.(*ast.Ident); ok {
			if *errname == "" {
				*errname = errv.Name
				return true
			}
			return errv.Name == *errname
		}
	}
	return false
}

// reports whether b contains a single return statement
// that is either naked, or that returns all zero values
// followed by a last expression (which is not further
// restricted); last is nil for naked returns.
func isReturn(b *ast.BlockStmt) (ok bool, last ast.Expr) {
	if len(b.List) != 1 {
		return
	}
	ret, _ := b.List[0].(*ast.ReturnStmt)
	if ret == nil {
		return
	}
	if n := len(ret.Results); n > 0 {
		for _, x := range ret.Results[:n-1] {
			if !isZero(x) {
				return
			}
		}
		return true, ret.Results[n-1]
	}
	return true, nil
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

// isName reports whether x is an identifier with the given name.
func isName(x ast.Expr, name string) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Name == name
}
