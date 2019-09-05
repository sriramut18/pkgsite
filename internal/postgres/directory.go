// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/lib/pq"
	"golang.org/x/discovery/internal"
	"golang.org/x/discovery/internal/derrors"
	"golang.org/x/discovery/internal/proxy"
	"golang.org/x/xerrors"
)

// GetDirectory returns the directory corresponding to the specified dirPath
// version. The directory will contain all packages for that version, in sorted
// order by package path. If version is empty, the directory corresponding to
// the latest matching module version will be fetched.
//
// Packages will be returned for a given dirPath if:
// (1) the package path has a prefix of dirPath+"/"
// (2) the dirPath has a prefix matching the package's module_path
//
// For example, if the package "golang.org/x/tools/go/packages" in module
// "golang.org/x/tools" is in the database, it will match on:
// golang.org/x/tools
// golang.org/x/tools/go
//
// It will not match on:
// golang.org/x/tools/g
// golang.org/x/tools/go/packages
//
// Additionally, if the package "github.com/hashicorp/vault/api" is in the
// database, and it is a package for the modules
// "github.com/hashicorp/vault/api" and "github.com/hashicorp/vault" it will
// only match for "github.com/hashicorp/vault".
func (db *DB) GetDirectory(ctx context.Context, dirPath, version string) (_ *internal.Directory, err error) {
	defer derrors.Wrap(&err, "DB.GetDirectory(ctx, %q, %q)", dirPath, version)

	query, args := constructDirectoryQueryAndArgs(dirPath, version)

	var packages []*internal.VersionedPackage
	collect := func(rows *sql.Rows) error {
		var (
			pkg                                 internal.VersionedPackage
			repositoryURL, vcsType, homepageURL sql.NullString
			licenseTypes, licensePaths          []string
		)
		if err := rows.Scan(&pkg.Path, &pkg.Version, &pkg.Name, &pkg.Synopsis, &pkg.V1Path,
			&pkg.DocumentationHTML, pq.Array(&licenseTypes),
			pq.Array(&licensePaths), &pkg.ModulePath, &pkg.ReadmeFilePath,
			&pkg.ReadmeContents, &pkg.CommitTime, &pkg.VersionType,
			&repositoryURL, &vcsType, &homepageURL); err != nil {
			return fmt.Errorf("row.Scan(): %v", err)
		}
		lics, err := zipLicenseMetadata(licenseTypes, licensePaths)
		if err != nil {
			return err
		}
		pkg.RepositoryURL = repositoryURL.String
		pkg.HomepageURL = homepageURL.String
		pkg.VCSType = vcsType.String
		pkg.Licenses = lics
		packages = append(packages, &pkg)
		return nil
	}
	if err := db.runQuery(ctx, query, collect, args...); err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return nil, xerrors.Errorf("packages in directory not found: %w", derrors.NotFound)
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Path < packages[j].Path
	})
	return &internal.Directory{
		Path:       dirPath,
		ModulePath: packages[0].ModulePath,
		Version:    packages[0].Version,
		Packages:   packages,
	}, nil
}

func constructDirectoryQueryAndArgs(dirPath, version string) (string, []interface{}) {
	baseQuery := `
		SELECT
			p.path,
			p.version,
			p.name,
			p.synopsis,
			p.v1_path,
			p.documentation,
			p.license_types,
			p.license_paths,
			p.module_path,
			v.readme_file_path,
			v.readme_contents,
			v.commit_time,
			v.version_type,
			v.repository_url,
			v.vcs_type,
			v.homepage_url
		FROM
			packages p`

	if version != proxy.Latest {
		return baseQuery + `
			INNER JOIN (
				SELECT *
				FROM versions
				WHERE
					version = $2
					AND $1 || '/' LIKE module_path || '/%'
			) v
			ON
				p.module_path = v.module_path
				AND v.version = p.version
			WHERE
				path LIKE $1 || '/%';`, []interface{}{dirPath, version}
	}

	return baseQuery + `
		INNER JOIN (
			SELECT
				DISTINCT ON (module_path) module_path,
				version,
				readme_file_path,
				readme_contents,
				commit_time,
				version_type,
				repository_url,
				vcs_type,
				homepage_url
			FROM
				versions
			WHERE
				$1 || '/' LIKE module_path || '/' || '%'
			ORDER BY
				-- Order the versions by release then prerelease.
				-- The default version should be the first release
				-- version available, if one exists.
				module_path,
				CASE WHEN prerelease = '~' THEN 0 ELSE 1 END,
				major DESC,
				minor DESC,
				patch DESC,
				prerelease DESC
		) v
		ON
			v.module_path = p.module_path
			AND v.version = p.version
		WHERE
			path LIKE $1 || '/' || '%';`, []interface{}{dirPath}
}
