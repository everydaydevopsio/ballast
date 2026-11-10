package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// installTypeScriptBackend installs the TypeScript backend into
// .ballast/tools/typescript.
//
// A released wrapper installs the published package. From a source checkout
// the backend is built and installed in isolation, for two reasons:
//
//   - `npm install <dir>` links the package directory without running any of
//     its scripts, and bin/ballast.js requires dist/cli, which only the build
//     produces. Installing an unbuilt or stale checkout replaced a working
//     backend with one that could not start, or kept serving old agents and
//     skills -- the "Unknown" agent of #417.
//   - The existing backend is usually that same link, so building in place
//     rewrites the live backend before the build is known to succeed: tsc
//     emits output even on type errors, and sync:content deletes and replaces
//     content.
//
// So the checkout's package directory is only ever read. The build happens in
// a staging directory, the staged package is packed and installed into a
// staging prefix, and only a prefix that installed cleanly is swapped into
// place. Every failure before the swap leaves the existing backend exactly as
// it was; a failed swap restores it.
func installTypeScriptBackend(version string, projectRoot string) error {
	toolRoot := filepath.Join(projectRoot, ".ballast", "tools", "typescript")
	sourceRoot := preferredInstallSourceRoot(projectRoot, version)
	if sourceRoot == "" {
		pkg := "@everydaydevopsio/ballast"
		if release := releaseVersion(version); release != "" {
			pkg += "@" + release
		}
		return runCommandFunc("npm", []string{"install", "--prefix", toolRoot, pkg})
	}

	if err := os.MkdirAll(filepath.Dir(toolRoot), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(toolRoot), err)
	}
	// Beside the destination, so the final swap is a rename on one filesystem.
	stage, err := os.MkdirTemp(filepath.Dir(toolRoot), "typescript-build-")
	if err != nil {
		return fmt.Errorf("create a TypeScript build directory: %w", err)
	}
	// Best effort: a leftover build directory costs disk, not correctness,
	// and the next install creates its own.
	defer func() { _ = os.RemoveAll(stage) }()

	pkgDir := filepath.Join(sourceRoot, "packages", "ballast-typescript")
	staged := filepath.Join(stage, "package")
	prefix := filepath.Join(stage, "prefix")

	// Dependencies only: node_modules at the pinned lockfile. Nothing the
	// backend serves is written here.
	if err := runCommandFunc("pnpm", []string{"--dir", sourceRoot, "install", "--frozen-lockfile"}); err != nil {
		return fmt.Errorf("install TypeScript build dependencies in %s: %w", sourceRoot, err)
	}
	if err := runCommandFunc("pnpm", []string{"--dir", pkgDir, "exec", "tsc", "--outDir", filepath.Join(staged, "dist")}); err != nil {
		return fmt.Errorf("build the TypeScript backend from %s: %w", pkgDir, err)
	}
	launcherTarget := filepath.Join(staged, "dist", "cli.js")
	if !fileExists(launcherTarget) {
		return fmt.Errorf("the TypeScript build finished but %s is missing; bin/ballast.js cannot start without it", launcherTarget)
	}
	// What sync:content would copy, without touching the checkout's copies.
	for _, dir := range []struct{ from, to string }{
		{filepath.Join(sourceRoot, "agents"), filepath.Join(staged, "agents")},
		{filepath.Join(sourceRoot, "skills"), filepath.Join(staged, "skills")},
		{filepath.Join(pkgDir, "bin"), filepath.Join(staged, "bin")},
	} {
		if err := copyTree(dir.from, dir.to); err != nil {
			return fmt.Errorf("stage the TypeScript backend: %w", err)
		}
	}
	if err := stagePackageManifest(filepath.Join(pkgDir, "package.json"), filepath.Join(staged, "package.json")); err != nil {
		return fmt.Errorf("stage the TypeScript backend: %w", err)
	}

	// Pack and install the tarball rather than the directory: a directory
	// install is a link, which is what tied the backend to the checkout.
	if err := runCommandFunc("npm", []string{"pack", staged, "--pack-destination", stage}); err != nil {
		return fmt.Errorf("pack the TypeScript backend: %w", err)
	}
	tarball, err := singleTarball(stage)
	if err != nil {
		return fmt.Errorf("pack the TypeScript backend: %w", err)
	}
	if err := runCommandFunc("npm", []string{"install", "--prefix", prefix, tarball}); err != nil {
		return fmt.Errorf("install the built TypeScript backend: %w", err)
	}
	if !fileExists(filepath.Join(prefix, "node_modules", ".bin", "ballast-typescript")) {
		return fmt.Errorf("the built TypeScript backend installed without its ballast-typescript launcher")
	}
	return swapInto(prefix, toolRoot)
}

// stagePackageManifest copies package.json without its scripts and development
// dependencies. npm pack runs prepack, and the checkout's prepack builds in
// place -- the thing this staging exists to avoid.
func stagePackageManifest(from, to string) error {
	raw, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return fmt.Errorf("parse %s: %w", from, err)
	}
	delete(manifest, "scripts")
	delete(manifest, "devDependencies")
	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, append(out, '\n'), 0o644)
}

func singleTarball(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.tgz"))
	if err != nil {
		return "", err
	}
	sort.Strings(matches)
	if len(matches) != 1 {
		return "", fmt.Errorf("expected one package tarball in %s, found %d", dir, len(matches))
	}
	return matches[0], nil
}

// swapInto replaces dest with next. The previous install is moved aside, not
// deleted, until next is in place, and is moved back if that fails.
func swapInto(next, dest string) error {
	previous := dest + ".previous"
	if err := os.RemoveAll(previous); err != nil {
		return fmt.Errorf("clear %s: %w", previous, err)
	}
	hadPrevious := true
	if err := os.Rename(dest, previous); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("move the existing TypeScript backend aside: %w", err)
		}
		hadPrevious = false
	}
	if err := os.Rename(next, dest); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(previous, dest); restoreErr != nil {
				return fmt.Errorf("install the TypeScript backend: %w (and restoring the previous one failed: %v)", err, restoreErr)
			}
		}
		return fmt.Errorf("install the TypeScript backend: %w", err)
	}
	if hadPrevious {
		_ = os.RemoveAll(previous)
	}
	return nil
}

// copyTree copies a directory of regular files, keeping their modes. Links and
// other special files are not expected in packaged content and are refused.
func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode().IsRegular():
			return copyFile(path, target, info.Mode().Perm())
		default:
			return fmt.Errorf("%s is not a regular file", path)
		}
	})
}

func copyFile(from, to string, mode os.FileMode) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
