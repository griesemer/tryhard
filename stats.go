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
	HasHandler
	HasElse
	TryCand
	numKinds = iota
)

var kindInfo = [numKinds]struct {
	report bool   // if set, calling count() records position information which is reported at the end
	desc   string // description
}{
	Func:       {false, "function declarations"},
	FuncError:  {false, "functions returning an error"},
	Stmt:       {false, "statements"},
	If:         {false, "if statements"},
	IfErr:      {false, "if <err> != nil statements"},
	NonErrName: {true, `<err> name is different from "err"`},
	ReturnErr:  {false, "return ..., <err> blocks in if <err> != nil statements"},
	HasHandler: {true, "more complex error handler in if <err> != nil statements; prevent use of try"},
	HasElse:    {true, "non-empty else blocks in if <err> != nil statements; prevent use of try"},
	TryCand:    {true, "try candidates"},
}

var counts [numKinds]int
var posLists [numKinds][]token.Pos

func count(k Kind, n ast.Node) {
	counts[k]++
	if kindInfo[k].report {
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
	reportCount(HasHandler, IfErr)
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
	if !*list && kindInfo[k].report {
		help = " (use -l flag to list file positions)"
	}
	fmt.Printf("% 7d (%5.1f%% of % 7d) %s%s\n", x, p, total, kindInfo[k].desc, help)
}

func reportPositions() {
	for k, list := range posLists {
		if len(list) == 0 {
			continue
		}
		fmt.Printf("--- %s ---\n", kindInfo[k].desc)
		for i, pos := range list {
			p := fset.Position(pos)
			fmt.Printf("% 7d  %s:%d\n", i+1, p.Filename, p.Line)
		}
		fmt.Println()
	}
}
