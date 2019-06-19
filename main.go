// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// tryhard is a very basic tool to list and rewrite `try` candidate statements.
// It operates on a file-by-file basis and does not type-check the code;
// potential statements are recognized and rewritten purely based on pattern
// matching, with the very real possibility of false positives. Use utmost
// caution when using the rewrite feature (-r flag) and have a backup as needed.
//
// Given a file, it operates on that file; given a directory, it operates on all
// .go files in that directory, recursively. Files starting with a period are ignored.
//
// tryhard considers each top-level function with a last result of named type `error`,
// which may or may not be the predefined type `error`. Inside these functions tryhard
// considers assignments of the form
//
//	v1, v2, ..., vn, <err> = f() // can also be := instead of =
//
// followed by an `if` statement of the form
//
//	if <err> != nil {
//		return ..., <err>
//	}
//
// or an `if` statement with an init expression matching the above assignment. The
// error variable <err> must be called "err", unless specified otherwise with the
// -err flag; the variable may or may not be of type `error` or correspond to the
// result error. The return statement must contain one or more return expressions,
// with all but the last one denoting a zero value of sorts (a zero number literal,
// an empty string, an empty composite literal, etc.). The last result must be the
// variable <err>.
//
// CAUTION: If the rewrite flag (-r) is specified, the file is updated in place!
//          Make sure you can revert to the original.
//
// Function literals (closures) are currently ignored.
//
// Usage:
//	tryhard [flags] [path ...]
//
// The flags are:
//	-l
//		list positions of potential `try` candidate statements
//	-r
//		rewrite potential `try` candidate statements to use `try`
//	-err
//		name of error variable; using "" permits any name
//
// If no -l is provided, tryhard reports the number of potential candidates found.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	// main operation modes
	list    = flag.Bool("l", false, "list positions of potential `try` candidate statements")
	rewrite = flag.Bool("r", false, "rewrite potential `try` candidate statements to use `try`")

	// customization
	varname = flag.String("err", "err", `name of error variable; using "" permits any name`)
)

var (
	fset     = token.NewFileSet()
	exitCode int
)

func report(err error) {
	scanner.PrintError(os.Stderr, err)
	exitCode = 2
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: tryhard [flags] [path ...]\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	for i := 0; i < flag.NArg(); i++ {
		path := flag.Arg(i)
		switch dir, err := os.Stat(path); {
		case err != nil:
			report(err)
		case dir.IsDir():
			filepath.Walk(path, visitFile)
		default:
			if err := processFile(path); err != nil {
				report(err)
			}
		}
	}

	if !*list {
		fmt.Println(count)
	}

	os.Exit(exitCode)
}

func isGoFile(f os.FileInfo) bool {
	// ignore non-Go files
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
}

func processFile(filename string) error {
	var perm os.FileMode = 0644
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	perm = fi.Mode().Perm()

	src, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	modified := false
	tryFile(file, &modified)
	if !modified || !*rewrite {
		return nil
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return err
	}
	res := buf.Bytes()

	// make a temporary backup before overwriting original
	bakname, err := backupFile(filename+".", src, perm)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, res, perm)
	if err != nil {
		os.Rename(bakname, filename)
		return err
	}
	return os.Remove(bakname)
}

func visitFile(path string, f os.FileInfo, err error) error {
	if err == nil && isGoFile(f) {
		err = processFile(path)
	}
	// Don't complain if a file was deleted in the meantime (i.e.
	// the directory changed concurrently while running tryhard).
	if err != nil && !os.IsNotExist(err) {
		report(err)
	}
	return nil
}

const chmodSupported = runtime.GOOS != "windows"

// backupFile writes data to a new file named filename<number> with permissions perm,
// with <number randomly chosen such that the file name is unique. backupFile returns
// the chosen file name.
func backupFile(filename string, data []byte, perm os.FileMode) (string, error) {
	// create backup file
	f, err := ioutil.TempFile(filepath.Dir(filename), filepath.Base(filename))
	if err != nil {
		return "", err
	}
	bakname := f.Name()
	if chmodSupported {
		err = f.Chmod(perm)
		if err != nil {
			f.Close()
			os.Remove(bakname)
			return bakname, err
		}
	}

	// write data to backup file
	_, err = f.Write(data)
	if err1 := f.Close(); err == nil {
		err = err1
	}

	return bakname, err
}
