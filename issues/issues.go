// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package issues is a mirror of the Go Issue Tracker
// for offline search and access.
package issues

import (
	"os"
	"path/filepath"

	"github.com/bradfitz/issuemirror"
)

// Open returns the root of the Go issue mirror cache,
// looking under $GOPATH/src/github.com/bradfitz/go-issue-mirror.
func Open() (issuemirror.Root, error) {
	root, err := goPackagePath("github.com/bradfitz/go-issue-mirror")
	if err != nil {
		return "", err
	}
	return issuemirror.Root(root), nil
}

// goPackagePath returns the path to the provided Go package's
// source directory.
// pkg may be a path prefix without any *.go files.
// The error is os.ErrNotExist if GOPATH is unset or the directory
// doesn't exist in any GOPATH component.
func goPackagePath(pkg string) (path string, err error) {
	gp := os.Getenv("GOPATH")
	if gp == "" {
		return path, os.ErrNotExist
	}
	for _, p := range filepath.SplitList(gp) {
		dir := filepath.Join(p, "src", filepath.FromSlash(pkg))
		fi, err := os.Stat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !fi.IsDir() {
			continue
		}
		return dir, nil
	}
	return path, os.ErrNotExist
}
