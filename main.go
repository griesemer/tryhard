// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// `tryhard` is a simple tool to list and rewrite `try` candidate statements.
// See README.md for details.
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
	"regexp"
	"runtime"
	"strings"
)

var (
	// main operation modes
	list    = flag.Bool("l", false, "list positions of potential `try` candidate statements")
	rewrite = flag.Bool("r", false, "rewrite potential `try` candidate statements to use `try`")

	// customization
	varname = flag.String("err", "", `name of error variable; using "" permits any name`)
	filter  = flag.String("ignore", "vendor", "ignore files with paths matching this (regexp) pattern")
)

var (
	fset      = token.NewFileSet()
	exitCode  int
	fileCount int
	filterRx  *regexp.Regexp
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

	if *filter != "" {
		rx, err := regexp.Compile(*filter)
		if err != nil {
			report(err)
			os.Exit(exitCode)
		}
		filterRx = rx
	}

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

	if fileCount > 0 {
		if *list {
			reportPositions()
		}
		reportCounts()
	}

	os.Exit(exitCode)
}

func processFile(filename string) error {
	fileCount++

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
	if err == nil && !excluded(path) && isGoFile(f) {
		err = processFile(path)
	}
	// Don't complain if a file was deleted in the meantime (i.e.
	// the directory changed concurrently while running tryhard).
	if err != nil && !os.IsNotExist(err) {
		report(err)
	}
	return nil
}

func excluded(path string) bool {
	return filterRx != nil && filterRx.MatchString(path)
}

func isGoFile(f os.FileInfo) bool {
	// ignore non-Go files
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
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
