package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// typeScriptSourceProject is a source checkout with a working TypeScript
// backend already installed, for the failure paths below.
func typeScriptSourceProject(t *testing.T) (root string, toolRoot string) {
	t.Helper()
	originalRun := runCommandFunc
	originalVersion := version
	t.Cleanup(func() {
		runCommandFunc = originalRun
		version = originalVersion
	})
	version = "dev"
	root = resolvedTempDir(t)
	ballastSourceCheckout(t, root, "5.22.0")
	staleTypeScriptBackend(t, root, "5.21.3")
	return root, filepath.Join(root, ".ballast", "tools", "typescript")
}

func launcherOf(toolRoot string) string {
	return filepath.Join(toolRoot, "node_modules", ".bin", "ballast-typescript")
}

// Every failure after the build but before the swap must leave the installed
// backend exactly as it was, and say which step failed.
func TestTypeScriptSourceInstallFailuresLeaveTheInstalledBackend(t *testing.T) {
	cases := map[string]struct {
		fail func(name string, args []string) error
		want string
	}{
		"pack fails": {
			fail: func(name string, args []string) error {
				if name == "npm" && args[0] == "pack" {
					return errors.New("npm ERR! pack")
				}
				return nil
			},
			want: "npm ERR! pack",
		},
		"pack writes no tarball": {
			fail: func(name string, args []string) error {
				if name == "npm" && args[0] == "pack" {
					return errSkipSimulation
				}
				return nil
			},
			want: "expected one package tarball",
		},
		"install fails": {
			fail: func(name string, args []string) error {
				if name == "npm" && args[0] == "install" {
					return errors.New("npm ERR! 404 yaml")
				}
				return nil
			},
			want: "npm ERR! 404 yaml",
		},
		"install leaves no launcher": {
			fail: func(name string, args []string) error {
				if name == "npm" && args[0] == "install" {
					return errSkipSimulation
				}
				return nil
			},
			want: "without its ballast-typescript launcher",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root, toolRoot := typeScriptSourceProject(t)
			before, _ := os.ReadFile(launcherOf(toolRoot))
			simulate := recordingTypeScriptBuild(t, nil)
			runCommandFunc = func(name string, args []string) error {
				if err := c.fail(name, args); err != nil {
					if errors.Is(err, errSkipSimulation) {
						return nil
					}
					return err
				}
				return simulate(name, args)
			}
			err := installTypeScriptBackend("dev", root)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
			after, _ := os.ReadFile(launcherOf(toolRoot))
			if string(after) != string(before) {
				t.Errorf("the installed backend changed: %q -> %q", before, after)
			}
			if leftovers, _ := filepath.Glob(filepath.Join(root, ".ballast", "tools", "typescript-build-*")); len(leftovers) != 0 {
				t.Errorf("the build directory was left behind: %v", leftovers)
			}
		})
	}
}

// errSkipSimulation makes a case's command succeed without producing its
// output, to exercise the checks that follow it.
var errSkipSimulation = errors.New("succeed without output")

// A backend built without the repository's agents or skills is the "Unknown"
// agent of #417 in another form. Refuse to build one.
func TestTypeScriptSourceInstallRefusesACheckoutWithoutContent(t *testing.T) {
	root, toolRoot := typeScriptSourceProject(t)
	if err := os.RemoveAll(filepath.Join(root, "skills")); err != nil {
		t.Fatal(err)
	}
	runCommandFunc = recordingTypeScriptBuild(t, nil)
	if err := installTypeScriptBackend("dev", root); err == nil || !strings.Contains(err.Error(), "skills") {
		t.Fatalf("err = %v, want the missing skills named", err)
	}
	if got, _ := os.ReadFile(launcherOf(toolRoot)); !strings.Contains(string(got), "5.21.3") {
		t.Errorf("the installed backend changed: %q", got)
	}
}

// The staged package carries the repository's current content and the
// checkout's launcher, and its manifest has no scripts for npm pack to run.
func TestTypeScriptSourceInstallStagesContentAndAScriptFreeManifest(t *testing.T) {
	root, _ := typeScriptSourceProject(t)
	mustWriteFile(t, filepath.Join(root, "packages", "ballast-typescript", "package.json"),
		`{"name":"@everydaydevopsio/ballast","version":"5.22.0","bin":{"ballast-typescript":"bin/ballast.js"},`+
			`"scripts":{"prepack":"pnpm run build"},"devDependencies":{"typescript":"^5"},"dependencies":{"yaml":"^2"}}`)
	var staged string
	simulate := recordingTypeScriptBuild(t, nil)
	runCommandFunc = func(name string, args []string) error {
		if name == "npm" && args[0] == "pack" {
			staged = args[1]
			for _, want := range []string{"agents/linting/content.md", "skills/example/SKILL.md", "bin/ballast.js", "dist/cli.js"} {
				if !fileExists(filepath.Join(staged, filepath.FromSlash(want))) {
					t.Errorf("staged package is missing %s", want)
				}
			}
			raw, err := os.ReadFile(filepath.Join(staged, "package.json"))
			if err != nil {
				t.Fatal(err)
			}
			var manifest map[string]any
			if err := json.Unmarshal(raw, &manifest); err != nil {
				t.Fatal(err)
			}
			if _, ok := manifest["scripts"]; ok {
				t.Error("the staged manifest still has scripts; npm pack would run prepack in place")
			}
			if _, ok := manifest["devDependencies"]; ok {
				t.Error("the staged manifest still has devDependencies")
			}
			if manifest["dependencies"] == nil || manifest["bin"] == nil {
				t.Errorf("the staged manifest lost its runtime fields: %v", manifest)
			}
		}
		return simulate(name, args)
	}
	if err := installTypeScriptBackend("dev", root); err != nil {
		t.Fatal(err)
	}
	if staged == "" {
		t.Fatal("nothing was packed")
	}
}

func TestStagePackageManifestRejectsAnUnreadableManifest(t *testing.T) {
	dir := t.TempDir()
	if err := stagePackageManifest(filepath.Join(dir, "missing.json"), filepath.Join(dir, "out.json")); err == nil {
		t.Error("accepted a missing manifest")
	}
	mustWriteFile(t, filepath.Join(dir, "bad.json"), "{not json")
	if err := stagePackageManifest(filepath.Join(dir, "bad.json"), filepath.Join(dir, "out.json")); err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("err = %v, want a parse error", err)
	}
}

func TestSingleTarballRefusesAmbiguity(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.tgz"), "a")
	mustWriteFile(t, filepath.Join(dir, "b.tgz"), "b")
	if _, err := singleTarball(dir); err == nil || !strings.Contains(err.Error(), "found 2") {
		t.Errorf("err = %v, want two tarballs refused", err)
	}
}

func TestSwapIntoReplacesAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	dest, next := filepath.Join(dir, "typescript"), filepath.Join(dir, "next")
	mustWriteFile(t, filepath.Join(dest, "old"), "old")
	mustWriteFile(t, filepath.Join(next, "new"), "new")
	if err := swapInto(next, dest); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(dest, "new")) || fileExists(filepath.Join(dest, "old")) {
		t.Error("dest does not hold the new install")
	}
	if fileExists(dest + ".previous") {
		t.Error("the previous install was left behind")
	}
}

func TestSwapIntoInstallsWhereNothingWasInstalled(t *testing.T) {
	dir := t.TempDir()
	dest, next := filepath.Join(dir, "typescript"), filepath.Join(dir, "next")
	mustWriteFile(t, filepath.Join(next, "new"), "new")
	if err := swapInto(next, dest); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(dest, "new")) {
		t.Error("dest does not hold the new install")
	}
}

// If the new install cannot be moved into place, the previous one goes back.
func TestSwapIntoRestoresThePreviousInstallWhenTheSwapFails(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "typescript")
	mustWriteFile(t, filepath.Join(dest, "old"), "old")
	err := swapInto(filepath.Join(dir, "does-not-exist"), dest)
	if err == nil {
		t.Fatal("a swap of a missing install succeeded")
	}
	if !fileExists(filepath.Join(dest, "old")) {
		t.Error("the previous install was not restored")
	}
}

// Packaged content is regular files; a link could point anywhere on the
// machine and would be packed as whatever it resolves to.
func TestCopyTreeRefusesLinks(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "from")
	mustWriteFile(t, filepath.Join(from, "ok.md"), "ok")
	if err := os.Symlink("/etc/hosts", filepath.Join(from, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := copyTree(from, filepath.Join(dir, "to")); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("err = %v, want the link refused", err)
	}
}
