// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// This file contains code patterns where `try` might
// be effective.
// TODO(gri) expand and use as actual tests for `try`.

package p

func f() error {
	return nil
}

type myError struct {
	error
}

func sharedReturnExpr() error {
	if err := f(); err != nil {
		return myError{err}
	}
	if err := f(); err != nil {
		return myError{err}
	}
}

func notSharedReturnExpr() error {
	if err := f(); err != nil {
		return myError{err}
	}
	if err := f(); err != nil {
		return myError{err}
	}
	if err := f(); err != nil {
		return err
	}
}

// f uses a function literal to annotate errors uniformly
func f(arg int) error {
	report := func(err error) error { return fmt.Errorf("f failed for %v: %v", arg, err) }

	err := g(arg)
	if err != nil {
		return report(err)
	}

	err = h(arg)
	if err != nil {
		return report(err)
	}
	return nil
}

// g uses repeated code (e.g., copy/paste) to annotate errors uniformly
func g(arg int) error {
	err := h(arg)
	if err != nil {
		return fmt.Errorf("g failed for %v: %v", arg, err)
	}

	err = f(arg)
	if err != nil {
		return fmt.Errorf("g failed for %v: %v", arg, err)
	}
	return nil
}

func h(arg int) error {
	return nil
}