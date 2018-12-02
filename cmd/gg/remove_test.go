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
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
	"gg-scm.io/pkg/internal/gittool"
)

func TestRemove(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with a committed foo.txt file.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Call gg to remove foo.txt.
	if _, err := env.gg(ctx, env.root.String(), "rm", "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify that foo.txt is not in the working copy.
	if exists, err := env.root.Exists("foo.txt"); err != nil {
		t.Error(err)
	} else if exists {
		t.Error("foo.txt exists after gg rm")
	}
	// Verify that foo.txt is no longer in the index.
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
		if ent.Name() != "foo.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'D' || code[1] != ' ' {
			t.Errorf("foo.txt status = '%v'; want 'D '", code)
		}
	}
	if !found {
		t.Error("File foo.txt unmodified")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_AddedFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with an uncommitted foo.txt.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Call gg to remove foo.txt. Verify that it returns an error.
	if _, err = env.gg(ctx, env.root.String(), "rm", "foo.txt"); err == nil {
		t.Error("`gg rm` returned success on added file")
	} else if isUsage(err) {
		t.Errorf("`gg rm` error: %v; want failure, not usage", err)
	}

	// Verify that foo.txt is still in the working copy.
	if exists, err := env.root.Exists("foo.txt"); err != nil {
		t.Error(err)
	} else if !exists {
		t.Error("foo.txt does not exist")
	}
	// Verify that foo.txt is still in the index as added.
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
		if ent.Name() != "foo.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'A' || code[1] != ' ' {
			t.Errorf("foo.txt status = '%v'; want 'A '", code)
		}
	}
	if !found {
		t.Error("File foo.txt removed")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_AddedForce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with an uncommitted foo.txt.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Call gg to remove foo.txt with the -f flag.
	if _, err := env.gg(ctx, env.root.String(), "rm", "-f", "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify that foo.txt is not in the working copy.
	if exists, err := env.root.Exists("foo.txt"); err != nil {
		t.Error(err)
	} else if exists {
		t.Error("foo.txt exists after gg rm")
	}
	// Verify that the index is clean.
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

func TestRemove_ModifiedFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with a committed foo.txt file.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Original Content\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	// Make local modifications to foo.txt.
	if err := env.root.Apply(filesystem.Write("foo.txt", "The world has changed...\n")); err != nil {
		t.Fatal(err)
	}

	// Call gg to remove foo.txt. Verify that it returns an error.
	if _, err = env.gg(ctx, env.root.String(), "rm", "foo.txt"); err == nil {
		t.Error("`gg rm` returned success on modified file")
	} else if isUsage(err) {
		t.Errorf("`gg rm` error: %v; want failure, not usage", err)
	}

	// Verify that foo.txt is still in the working copy.
	if exists, err := env.root.Exists("foo.txt"); err != nil {
		t.Error(err)
	} else if !exists {
		t.Error("foo.txt does not exist")
	}
	// Verify that foo.txt is still in the index as modified.
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
		if ent.Name() != "foo.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != ' ' || code[1] != 'M' {
			t.Errorf("foo.txt status = '%v'; want ' M'", code)
		}
	}
	if !found {
		t.Error("File foo.txt reverted")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_ModifiedForce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with a committed foo.txt file.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Original Content\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	// Make local modifications to foo.txt.
	if err := env.root.Apply(filesystem.Write("foo.txt", "The world has changed...\n")); err != nil {
		t.Fatal(err)
	}

	// Call gg to remove foo.txt with the -f flag.
	if _, err := env.gg(ctx, env.root.String(), "rm", "-f", "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify that foo.txt is not in the working copy.
	if exists, err := env.root.Exists("foo.txt"); err != nil {
		t.Error(err)
	} else if exists {
		t.Error("foo.txt exists after gg rm")
	}
	// Verify that foo.txt is no longer in the index.
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
		if ent.Name() != "foo.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'D' || code[1] != ' ' {
			t.Errorf("foo.txt status = '%v'; want 'D '", code)
		}
	}
	if !found {
		t.Error("File foo.txt unmodified")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_MissingFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with a committed foo.txt file.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	// Remove the foo.txt file without informing Git.
	if err := env.root.Apply(filesystem.Remove("foo.txt")); err != nil {
		t.Fatal(err)
	}

	// Call gg to remove foo.txt. Verify that gg returns an error.
	if _, err = env.gg(ctx, env.root.String(), "rm", "foo.txt"); err == nil {
		t.Error("`gg rm` returned success on missing file")
	} else if isUsage(err) {
		t.Errorf("`gg rm` error: %v; want failure, not usage", err)
	}
	// Verify that foo.txt is still in the index.
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
		if ent.Name() != "foo.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != ' ' || code[1] != 'D' {
			t.Errorf("foo.txt status = '%v'; want ' D'", code)
		}
	}
	if !found {
		t.Error("File foo.txt reverted")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_MissingAfter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with a committed foo.txt file.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	// Remove the foo.txt file without informing Git.
	if err := env.root.Apply(filesystem.Remove("foo.txt")); err != nil {
		t.Fatal(err)
	}

	// Call gg to remove foo.txt with the -after flag.
	if _, err := env.gg(ctx, env.root.String(), "rm", "-after", "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify that foo.txt is no longer in the index.
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
		if ent.Name() != "foo.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'D' || code[1] != ' ' {
			t.Errorf("foo.txt status = '%v'; want 'D '", code)
		}
	}
	if !found {
		t.Error("File foo.txt reverted")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_Recursive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with a committed foo/bar.txt file.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo/bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo/bar.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Call gg to remove the foo directory.
	if _, err := env.gg(ctx, env.root.String(), "rm", "-r", "foo"); err != nil {
		t.Error(err)
	}

	// Verify that foo/bar.txt is not in the working copy.
	if exists, err := env.root.Exists("foo/bar.txt"); err != nil {
		t.Error(err)
	} else if exists {
		t.Error("foo/bar.txt exists after gg rm")
	}
	// Verify that foo/bar.txt is not in the index.
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
		if ent.Name() != "foo/bar.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'D' || code[1] != ' ' {
			t.Errorf("foo/bar.txt status = '%v'; want 'D '", code)
		}
	}
	if !found {
		t.Error("File foo/bar.txt unmodified")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_RecursiveMissingFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with a committed foo/bar.txt file.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo/bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo/bar.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	// Remove the directory without informing Git.
	if err := env.root.Apply(filesystem.Remove("foo")); err != nil {
		t.Fatal(err)
	}

	// Call gg to remove the foo directory. Verify that gg returns an error.
	if _, err := env.gg(ctx, env.root.String(), "rm", "-r", "foo"); err == nil {
		t.Error("`gg rm -r` returned success on missing directory")
	} else if isUsage(err) {
		t.Errorf("`gg rm -r` error: %v; want failure, not usage", err)
	}

	// Verify that foo.txt is still in the index.
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
		if ent.Name() != "foo/bar.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != ' ' || code[1] != 'D' {
			t.Errorf("foo/bar.txt status = '%v'; want ' D'", code)
		}
	}
	if !found {
		t.Error("File foo/bar.txt unmodified")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_RecursiveMissingAfter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with a committed foo/bar.txt file.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo/bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo/bar.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	// Remove the directory without informing Git.
	if err := env.root.Apply(filesystem.Remove("foo")); err != nil {
		t.Fatal(err)
	}

	// Call gg to remove the foo directory with the -after flag.
	if _, err := env.gg(ctx, env.root.String(), "rm", "-r", "-after", "foo"); err != nil {
		t.Error(err)
	}

	// Verify that foo/bar.txt is not in the index.
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
		if ent.Name() != "foo/bar.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'D' || code[1] != ' ' {
			t.Errorf("foo/bar.txt status = '%v'; want 'D '", code)
		}
	}
	if !found {
		t.Error("File foo/bar.txt unmodified")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}
