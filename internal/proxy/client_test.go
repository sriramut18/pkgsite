// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/discovery/internal/derrors"
	"golang.org/x/xerrors"
)

const testTimeout = 5 * time.Second

func TestGetLatestInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	modulePath := "foo.com/bar"
	testVersions := []*TestVersion{
		NewTestVersion(t, "foo.com/bar", "v1.1.0", map[string]string{"bar.go": "package bar\nconst Version = 1.1"}),
		NewTestVersion(t, "foo.com/bar", "v1.2.0", map[string]string{"bar.go": "package bar\nconst Version = 1.2"}),
	}

	client, teardownProxy := SetupTestProxy(t, testVersions)
	defer teardownProxy()

	info, err := client.GetInfo(ctx, modulePath, Latest)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := info.Version, "v1.2.0"; got != want {
		t.Errorf("GetLatestInfo(ctx, %q): Version = %q, want %q", modulePath, got, want)
	}
}

func TestListVersions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	modulePath := "foo.com/bar"
	testVersions := []*TestVersion{
		NewTestVersion(t, "foo.com/bar", "v1.1.0", map[string]string{"bar.go": "package bar\nconst Version = 1.1"}),
		NewTestVersion(t, "foo.com/bar", "v1.2.0", map[string]string{"bar.go": "package bar\nconst Version = 1.2"}),
		NewTestVersion(t, "foo.com/baz", "v1.3.0", map[string]string{"baz.go": "package bar\nconst Version = 1.3"}),
	}

	client, teardownProxy := SetupTestProxy(t, testVersions)
	defer teardownProxy()

	want := []string{"v1.1.0", "v1.2.0"}
	got, err := client.ListVersions(ctx, modulePath)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListVersions(%q) diff:\n%s", modulePath, diff)
	}
}

func TestGetInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client, teardownProxy := SetupTestProxy(t, nil)
	defer teardownProxy()

	path := "github.com/my/module"
	version := "v1.0.0"
	info, err := client.GetInfo(ctx, path, version)
	if err != nil {
		t.Fatal(err)
	}

	if info.Version != version {
		t.Errorf("VersionInfo.Version for GetInfo(ctx, %q, %q) = %q, want %q", path, version, info.Version, version)
	}

	expectedTime := time.Date(2019, 1, 30, 0, 0, 0, 0, time.UTC)
	if info.Time != expectedTime {
		t.Errorf("VersionInfo.Time for GetInfo(ctx, %q, %q) = %v, want %v", path, version, info.Time, expectedTime)
	}
}

func TestGetInfoVersionDoesNotExist(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client, teardownProxy := SetupTestProxy(t, nil)
	defer teardownProxy()

	path := "github.com/my/module"
	version := "v3.0.0"
	info, _ := client.GetInfo(ctx, path, version)
	if info != nil {
		t.Errorf("GetInfo(ctx, %q, %q) = %v, want %v", path, version, info, nil)
	}
}

func TestGetZip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client, teardownProxy := SetupTestProxy(t, nil)
	defer teardownProxy()

	for _, tc := range []struct {
		path, version string
		wantFiles     []string
	}{
		{
			path:    "github.com/my/module",
			version: "v1.0.0",
			wantFiles: []string{
				"github.com/my/module@v1.0.0/LICENSE",
				"github.com/my/module@v1.0.0/README.md",
				"github.com/my/module@v1.0.0/go.mod",
				"github.com/my/module@v1.0.0/foo/foo.go",
				"github.com/my/module@v1.0.0/foo/LICENSE.md",
				"github.com/my/module@v1.0.0/bar/bar.go",
				"github.com/my/module@v1.0.0/bar/LICENSE",
			},
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			zipReader, err := client.GetZip(ctx, tc.path, tc.version)
			if err != nil {
				t.Fatal(err)
			}

			if len(zipReader.File) != len(tc.wantFiles) {
				t.Errorf("GetZip(ctx, %q, %q) returned number of files: got %d, want %d",
					tc.path, tc.version, len(zipReader.File), len(tc.wantFiles))
			}

			expectedFileSet := map[string]bool{}
			for _, ef := range tc.wantFiles {
				expectedFileSet[ef] = true
			}
			for _, zipFile := range zipReader.File {
				if !expectedFileSet[zipFile.Name] {
					t.Errorf("GetZip(ctx, %q, %q) returned unexpected file: %q", tc.path,
						tc.version, zipFile.Name)
				}
				expectedFileSet[zipFile.Name] = false
			}
		})
	}
}

func TestGetZipNonExist(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	client, teardownProxy := SetupTestProxy(t, nil)
	defer teardownProxy()

	path := "my.mod/nonexistmodule"
	version := "v1.0.0"
	if _, err := client.GetZip(ctx, path, version); !xerrors.Is(err, derrors.NotFound) {
		t.Errorf("got %v, want %v", err, derrors.NotFound)
	}
}
