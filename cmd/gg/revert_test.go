// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"path/filepath"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
	"gg-scm.io/pkg/internal/gittool"
)

func TestRevert(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		dir          string
		stagedPath   string
		unstagedPath string
	}{
		{
			name:         "TopLevel",
			dir:          ".",
			stagedPath:   "staged.txt",
			unstagedPath: "unstaged.txt",
		},
		{
			name:         "FromSubdir",
			dir:          "foo",
			stagedPath:   filepath.Join("..", "staged.txt"),
			unstagedPath: filepath.Join("..", "unstaged.txt"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()

			// Create a working copy where both staged.txt and unstaged.txt
			// have local modifications but only staged.txt has been "git add"ed.
			if err := env.initEmptyRepo(ctx, "."); err != nil {
				t.Fatal(err)
			}
			err = env.root.Apply(
				filesystem.Mkdir("foo"),
				filesystem.Write("staged.txt", "ohai"),
				filesystem.Write("unstaged.txt", "kthxbai"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := env.trackFiles(ctx, "staged.txt", "unstaged.txt"); err != nil {
				t.Fatal(err)
			}
			if _, err := env.newCommit(ctx, "."); err != nil {
				t.Fatal(err)
			}
			err = env.root.Apply(
				filesystem.Write("staged.txt", "mumble mumble 1"),
				filesystem.Write("unstaged.txt", "mumble mumble 2"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := env.addFiles(ctx, "staged.txt"); err != nil {
				t.Fatal(err)
			}

			// Call gg to revert the staged file.
			if _, err := env.gg(ctx, env.root.FromSlash(test.dir), "revert", test.stagedPath); err != nil {
				t.Fatal(err)
			}
			// Verify that staged.txt has the original content and the
			// modified content was saved to staged.txt.orig.
			if got, err := env.root.ReadFile("staged.txt"); err != nil {
				t.Error(err)
			} else if want := "ohai"; got != want {
				t.Errorf("staged.txt content = %q after revert; want %q", got, want)
			}
			if got, err := env.root.ReadFile("staged.txt.orig"); err != nil {
				t.Error(err)
			} else if want := "mumble mumble 1"; got != want {
				t.Errorf("staged.txt.orig content = %q after revert; want %q", got, want)
			}

			// Call gg to revert the unstaged file.
			if _, err := env.gg(ctx, env.root.FromSlash(test.dir), "revert", test.unstagedPath); err != nil {
				t.Fatal(err)
			}
			if got, err := env.root.ReadFile("unstaged.txt"); err != nil {
				t.Error(err)
			} else if want := "kthxbai"; got != want {
				t.Errorf("unstaged.txt content = %q after revert; want %q", got, want)
			}
			// Verify that unstaged.txt has the original content and the
			// modified content was saved to unstaged.txt.orig.
			if got, err := env.root.ReadFile("unstaged.txt.orig"); err != nil {
				t.Error(err)
			} else if want := "mumble mumble 2"; got != want {
				t.Errorf("unstaged.txt.orig content = %q after revert; want %q", got, want)
			}
			if got, err := env.root.ReadFile("staged.txt"); err != nil {
				t.Error(err)
			} else if want := "ohai"; got != want {
				t.Error("unrelated file was reverted")
			}

			// Verify that working copy is clean (sans backup files).
			st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := st.Close(); err != nil {
					t.Error("st.Close():", err)
				}
			}()
			for st.Scan() {
				if name := st.Entry().Name(); name == "staged.txt.orig" || name == "unstaged.txt.orig" {
					continue
				}
				t.Errorf("Found status: %v; want clean working copy", st.Entry())
			}
			if err := st.Err(); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestRevert_AddedFile(t *testing.T) {
	t.Parallel()
	for _, backup := range []bool{true, false} {
		var name string
		if backup {
			name = "WithBackup"
		} else {
			name = "NoBackup"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()

			// Create a repository with foo.txt "git add -N"ed.
			if err := env.initRepoWithHistory(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.root.Apply(filesystem.Write("foo.txt", "hey there")); err != nil {
				t.Fatal(err)
			}
			if err := env.trackFiles(ctx, "foo.txt"); err != nil {
				t.Fatal(err)
			}

			// Call gg to revert the added file.
			revertArgs := []string{"revert"}
			if !backup {
				revertArgs = append(revertArgs, "--no-backup")
			}
			revertArgs = append(revertArgs, "foo.txt")
			if _, err := env.gg(ctx, env.root.String(), revertArgs...); err != nil {
				t.Fatal(err)
			}

			// Verify that foo.txt still exists and has the same content.
			if got, err := env.root.ReadFile("foo.txt"); err != nil {
				t.Error(err)
			} else if want := "hey there"; got != want {
				t.Errorf("content = %q after revert; want %q", got, want)
			}
			// Verify that foo.txt.orig was not created.
			if exists, err := env.root.Exists("foo.txt.orig"); err != nil {
				t.Error(err)
			} else if exists {
				t.Error("foo.txt.orig was created")
			}
			// Verify that foo.txt is untracked.
			st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := st.Close(); err != nil {
					t.Error("st.Close():", err)
				}
			}()
			for st.Scan() {
				ent := st.Entry()
				switch ent.Name() {
				case "foo.txt":
					if got := ent.Code(); !got.IsUntracked() {
						t.Errorf("foo.txt status code = '%v'; want '??'", got)
					}
				case "foo.txt.orig":
					// Ignore, error already reported.
				default:
					t.Errorf("Found status: %v; want untracked foo.txt", st.Entry())
				}
			}
			if err := st.Err(); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestRevert_AddedFileBeforeFirstCommit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with foo.txt "git add -N"ed.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "ohai")); err != nil {
		t.Fatal(err)
	}
	if err := env.trackFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Call gg to revert foo.txt.
	if _, err := env.gg(ctx, env.root.String(), "revert", "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify that foo.txt still exists and has the same content.
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "ohai"; got != want {
		t.Errorf("content = %q after revert; want %q", got, want)
	}
	// Verify that foo.txt.orig was not created.
	if exists, err := env.root.Exists("foo.txt.orig"); err != nil {
		t.Error(err)
	} else if exists {
		t.Error("foo.txt.orig was created")
	}
	// Verify that foo.txt is untracked.
	st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Error("st.Close():", err)
		}
	}()
	for st.Scan() {
		ent := st.Entry()
		switch ent.Name() {
		case "foo.txt":
			if got := ent.Code(); !got.IsUntracked() {
				t.Errorf("foo.txt status code = '%v'; want '??'", got)
			}
		case "foo.txt.orig":
			// Ignore, error already reported.
		default:
			t.Errorf("Found status: %v; want untracked foo.txt", st.Entry())
		}
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRevert_All(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a working copy where both staged.txt and unstaged.txt
	// have local modifications but only staged.txt has been "git add"ed.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	err = env.root.Apply(
		filesystem.Write("staged.txt", "original stage"),
		filesystem.Write("unstaged.txt", "original audience"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "staged.txt", "unstaged.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	err = env.root.Apply(
		filesystem.Write("staged.txt", "randomness"),
		filesystem.Write("unstaged.txt", "randomness"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "staged.txt"); err != nil {
		t.Fatal(err)
	}

	// Call gg to revert everything.
	if _, err := env.gg(ctx, env.root.String(), "revert", "--all"); err != nil {
		t.Fatal(err)
	}
	// Verify that staged.txt has the original content.
	if got, err := env.root.ReadFile("staged.txt"); err != nil {
		t.Error(err)
	} else if want := "original stage"; got != want {
		t.Errorf("staged modified file content = %q after revert; want %q", got, want)
	}
	// Verify that unstaged.txt has the original content.
	if got, err := env.root.ReadFile("unstaged.txt"); err != nil {
		t.Error(err)
	} else if want := "original audience"; got != want {
		t.Errorf("unstaged modified file content = %q after revert; want %q", got, want)
	}
	// Verify that working copy is clean (sans backup files).
	st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Error("st.Close():", err)
		}
	}()
	for st.Scan() {
		if name := st.Entry().Name(); name == "staged.txt.orig" || name == "unstaged.txt.orig" {
			continue
		}
		t.Errorf("Found status: %v; want clean working copy", st.Entry())
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRevert_Rev(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository that has two commits of foo.txt.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "original content")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "super-fresh content")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Call gg to revert foo.txt to the first commit's content.
	if _, err := env.gg(ctx, env.root.String(), "revert", "-r", "HEAD^", "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify foo.txt now has the same content as in the first commit.
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "original content"; got != want {
		t.Errorf("foo.txt content = %q after revert; want %q", got, want)
	}
	// Verify that foo.txt.orig was not created.
	if exists, err := env.root.Exists("foo.txt.orig"); err != nil {
		t.Error(err)
	} else if exists {
		t.Error("foo.txt.orig was created")
	}
	// Verify that Git considers foo.txt locally modified.
	st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Error("st.Close():", err)
		}
	}()
	found := false
	for st.Scan() {
		ent := st.Entry()
		switch ent.Name() {
		case "foo.txt":
			found = true
			if code := ent.Code(); !(code[0] == ' ' && code[1] == 'M') && !(code[0] == 'M' || code[1] == ' ') {
				t.Errorf("foo.txt status = '%v'; want ' M' or 'M '", code)
			}
		case "foo.txt.orig":
			// Error already reported.
		default:
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
	}
	if !found {
		t.Error("File foo.txt unmodified")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRevert_Missing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository and commit foo.txt.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "original content")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	// Remove foo.txt without informing Git.
	if err := env.root.Apply(filesystem.Remove("foo.txt")); err != nil {
		t.Fatal(err)
	}

	// Call gg to revert foo.txt.
	if _, err := env.gg(ctx, env.root.String(), "revert", "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify that foo.txt exists now.
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "original content"; got != want {
		t.Errorf("file content = %q after revert; want %q", got, want)
	}
	// Verify that the working copy is clean.
	st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Error("st.Close():", err)
		}
	}()
	for st.Scan() {
		t.Errorf("Found status: %v; want clean working copy", st.Entry())
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRevert_NoBackup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with a committed foo.txt.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "original content")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	// Modify foo.txt in the working copy.
	if err := env.root.Apply(filesystem.Write("foo.txt", "tears in rain")); err != nil {
		t.Fatal(err)
	}

	// Call gg to revert foo.txt without backups.
	if _, err := env.gg(ctx, env.root.String(), "revert", "--no-backup", "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify that foo.txt has the committed content.
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "original content"; got != want {
		t.Errorf("file content = %q after revert; want %q", got, want)
	}
	// Verify that foo.txt.orig was not created.
	if exists, err := env.root.Exists("foo.txt.orig"); err != nil {
		t.Error(err)
	} else if exists {
		t.Error("foo.txt.orig was created")
	}
}

func TestRevert_LocalRename(t *testing.T) {
	// The `git status` that gets reported here is a little weird on newer
	// versions of Git. This makes sure that revert doesn't do something
	// naive.

	t.Parallel()
	tests := []struct {
		name          string
		revertFoo     bool
		revertRenamed bool
	}{
		{name: "RevertOriginal", revertFoo: true},
		{name: "RevertRenamed", revertRenamed: true},
		{name: "RevertBoth", revertFoo: true, revertRenamed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()

			// Create a repository with a committed foo.txt.
			if err := env.initEmptyRepo(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.root.Apply(filesystem.Write("foo.txt", "original content")); err != nil {
				t.Fatal(err)
			}
			if err := env.addFiles(ctx, "foo.txt"); err != nil {
				t.Fatal(err)
			}
			if _, err := env.newCommit(ctx, "."); err != nil {
				t.Fatal(err)
			}
			// Move from foo.txt to renamed.txt and "git add -N" renamed.txt.
			// This effectively makes foo.txt missing.
			if err := env.root.Apply(filesystem.Rename("foo.txt", "renamed.txt")); err != nil {
				t.Fatal(err)
			}
			if err := env.trackFiles(ctx, "renamed.txt"); err != nil {
				t.Fatal(err)
			}

			// Call gg to revert foo.txt and/or renamed.txt.
			revertArgs := []string{"revert"}
			if test.revertFoo {
				revertArgs = append(revertArgs, "foo.txt")
			}
			if test.revertRenamed {
				revertArgs = append(revertArgs, "renamed.txt")
			}
			if _, err := env.gg(ctx, env.root.String(), revertArgs...); err != nil {
				t.Fatal(err)
			}

			if test.revertFoo {
				// Verify that foo.txt matches the committed content.
				if got, err := env.root.ReadFile("foo.txt"); err != nil {
					t.Error(err)
				} else if want := "original content"; got != want {
					t.Errorf("foo.txt content = %q after revert; want %q", got, want)
				}
			} else {
				// Verify that foo.txt doesn't exist.
				if exists, err := env.root.Exists("foo.txt"); err != nil {
					t.Error(err)
				} else if exists {
					t.Error("foo.txt was created")
				}
			}
			// Regardless, verify that a backup for renamed.txt was not created.
			if exists, err := env.root.Exists("renamed.txt.orig"); err != nil {
				t.Error(err)
			} else if exists {
				t.Error("renamed.txt.orig was created")
			}
			// Verify status renamed.txt matches expectations.
			st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := st.Close(); err != nil {
					t.Error("st.Close():", err)
				}
			}()
			for st.Scan() {
				ent := st.Entry()
				switch ent.Name() {
				case "renamed.txt":
					if got := ent.Code(); test.revertRenamed && !got.IsUntracked() {
						t.Errorf("renamed.txt status code = '%v'; want '??'", got)
					} else if !test.revertRenamed && !got.IsAdded() {
						t.Errorf("renamed.txt status code = '%v'; want to contain 'A'", got)
					}
				case "foo.txt", "foo.txt.orig", "renamed.txt.orig":
					// Ignore, error already reported if needed.
				default:
					t.Errorf("Found status: %v; want untracked renamed.txt", st.Entry())
				}
			}
			if err := st.Err(); err != nil {
				t.Error(err)
			}
		})
	}
}
