// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/ast"
	"go/token"
)

type Kind int

const (
	Func = Kind(iota)
	FuncError
	Stmt
	If
	IfErr
	TryCand
	NonErrName

	// non-try candidates
	ReturnExpr
	SingleStmt
	ComplexBlock
	HasElse

	numKinds = iota
)

var kindDesc = [numKinds]string{
	Func:       "functions (function literals are ignored)",
	FuncError:  "functions returning an error",
	Stmt:       "statements in functions returning an error",
	If:         "if statements",
	IfErr:      "if <err> != nil statements",
	TryCand:    "try candidates",
	NonErrName: `<err> name is different from "err"`,

	// non-try candidates
	ReturnExpr:   "{ return ... zero values ..., expr }",
	SingleStmt:   "single statement then branch",
	ComplexBlock: "complex then branch; cannot use try",
	HasElse:      "non-empty else branch; cannot use try",
}

var counts [numKinds]int
var posLists [numKinds][]token.Pos

// count adds 1 to the number of nodes categorized as k.
// Counts are reported with reportCounts.
// If n != nil, count appends that node's position to the
// list of positions categorized as k. Collected positions
// are reported with reportPositions.
func count(k Kind, n ast.Node) {
	counts[k]++
	if n != nil {
		posLists[k] = append(posLists[k], n.Pos())
	}
}

func reportCounts() {
	fmt.Println("--- stats ---")
	reportCount(Func, Func)
	reportCount(FuncError, Func)
	reportCount(Stmt, Stmt)
	reportCount(If, Stmt)
	reportCount(IfErr, If)
	reportCount(TryCand, IfErr)
	reportCount(NonErrName, IfErr)

	help := ""
	if !*list {
		help = " (-l flag lists file positions)"
	}
	fmt.Printf("--- non-try candidates%s ---\n", help)
	reportCount(ReturnExpr, IfErr)
	reportCount(SingleStmt, IfErr)
	reportCount(ComplexBlock, IfErr)
	reportCount(HasElse, IfErr)
}

func reportCount(k, ofk Kind) {
	x := counts[k]
	total := counts[ofk]
	// don't crash if total == 0
	p := 100.0 // percentage
	if total != 0 {
		p = float64(x) * 100 / float64(total)
	}
	fmt.Printf("% 7d (%5.1f%% of % 7d) %s\n", x, p, total, kindDesc[k])
}

func reportPositions() {
	for k, list := range posLists {
		if len(list) == 0 {
			continue
		}
		fmt.Printf("--- %s ---\n", kindDesc[k])
		for i, pos := range list {
			p := fset.Position(pos)
			fmt.Printf("% 7d  %s:%d\n", i+1, p.Filename, p.Line)
		}
		fmt.Println()
	}
}
