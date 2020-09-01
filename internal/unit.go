// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"time"

	"github.com/google/safehtml"
	"golang.org/x/pkgsite/internal/licenses"
	"golang.org/x/pkgsite/internal/source"
)

// PathInfo represents metadata about a unit.
type PathInfo struct {
	// Unit level information
	//
	Path              string
	Name              string
	IsRedistributable bool
	Licenses          []*licenses.Metadata

	// Module level information
	//
	Version    string
	ModulePath string
	CommitTime time.Time
	SourceInfo *source.Info
}

// IsPackage reports whether the path represents a package path.
func (pi *PathInfo) IsPackage() bool {
	return pi.Name != ""
}

// IsModule reports whether the path represents a module path.
func (pi *PathInfo) IsModule() bool {
	return pi.ModulePath == pi.Path
}

// Unit represents the contents of some path in the Go package/module
// namespace. It might be a module, a package, both a module and a package, or
// none of the above: a directory within a module that has no .go files, but
// contains other units, licenses and/or READMEs."
type Unit struct {
	PathInfo
	Readme  *Readme
	Package *Package
	Imports []string
}

// Documentation is the rendered documentation for a given package
// for a specific GOOS and GOARCH.
type Documentation struct {
	// The values of the GOOS and GOARCH environment variables used to parse the
	// package.
	GOOS     string
	GOARCH   string
	Synopsis string
	HTML     safehtml.HTML
}

// Readme is a README at the specified filepath.
type Readme struct {
	Filepath string
	Contents string
}
