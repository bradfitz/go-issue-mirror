// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package issues is a mirror of the Go Issue Tracker
// for offline search and access.
package issues

import (
	"go/build"

	"github.com/bradfitz/issuemirror"
)

// Open returns the root of the Go issue mirror cache,
// looking under $GOPATH/src/github.com/bradfitz/go-issue-mirror/_data.
func Open() (issuemirror.Root, error) {
	root, err := importPathToDir("github.com/bradfitz/go-issue-mirror/_data")
	if err != nil {
		return "", err
	}
	return issuemirror.Root(root), nil
}

// importPathToDir resolves the absolute path from importPath.
// There doesn't need to be a valid Go package inside that import path,
// but the directory must exist.
func importPathToDir(importPath string) (string, error) {
	p, err := build.Import(importPath, "", build.FindOnly)
	if err != nil {
		return "", err
	}
	return p.Dir, nil
}
