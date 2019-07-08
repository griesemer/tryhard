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
	NonErrName
	ReturnErr
	ReturnExpr
	ComplexBlock
	HasElse
	TryCand
	numKinds = iota
)

var kindDesc = [numKinds]string{
	Func:         "func declarations",
	FuncError:    "func declarations returning an error",
	Stmt:         "statements",
	If:           "if statements",
	IfErr:        "if <err> != nil statements",
	NonErrName:   `<err> name is different from "err"`,
	ReturnErr:    "{ return ..., <err> } in if <err> != nil statements",
	ReturnExpr:   "{ return ..., expr } in if <err> != nil statements",
	ComplexBlock: "complex handler in if <err> != nil statements; cannot use try",
	HasElse:      "non-empty else in if <err> != nil statements; cannot use try",
	TryCand:      "try candidates",
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
	fmt.Printf("--- stats ---\n")
	reportCount(Func, Func)
	reportCount(FuncError, Func)
	reportCount(Stmt, Stmt)
	reportCount(If, Stmt)
	reportCount(IfErr, If)
	reportCount(NonErrName, IfErr)
	reportCount(ReturnErr, IfErr)
	reportCount(ReturnExpr, IfErr)
	reportCount(ComplexBlock, IfErr)
	reportCount(HasElse, IfErr)
	reportCount(TryCand, IfErr)
}

func reportCount(k, ofk Kind) {
	x := counts[k]
	total := counts[ofk]
	// don't crash if total == 0
	p := 100.0 // percentage
	if total != 0 {
		p = float64(x) * 100 / float64(total)
	}
	help := ""
	if !*list && len(posLists[k]) != 0 {
		help = " (-l flag lists details)"
	}
	fmt.Printf("% 7d (%5.1f%% of % 7d) %s%s\n", x, p, total, kindDesc[k], help)
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
