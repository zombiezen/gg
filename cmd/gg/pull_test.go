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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"zombiezen.com/go/gg/internal/gittool"
)

func TestPull(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	pullEnv, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.gg(ctx, pullEnv.repoB, "pull"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pullEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() != pullEnv.commit1 {
			t.Errorf("HEAD = %s; want %s", r.CommitHex(), pullEnv.commit1)
		}
		if r.RefName() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.RefName())
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "origin/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pullEnv.commit1 {
			t.Errorf("origin/master = %s (first commit); want %s", r.CommitHex(), pullEnv.commit2)
		} else if r.CommitHex() != pullEnv.commit2 {
			t.Errorf("origin/master = %s; want %s", r.CommitHex(), pullEnv.commit2)
		}
	}
}

func TestPullWithArgument(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	pullEnv, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	// Move HEAD away.  We want to check that the corresponding branch is pulled.
	if err := env.git.WithDir(pullEnv.repoA).Run(ctx, "checkout", "--detach", "HEAD^"); err != nil {
		t.Fatal(err)
	}
	repoC := filepath.Join(env.root, "repoC")
	if err := os.Rename(pullEnv.repoA, repoC); err != nil {
		t.Fatal(err)
	}
	if err := env.gg(ctx, pullEnv.repoB, "pull", repoC); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pullEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() != pullEnv.commit1 {
			t.Errorf("HEAD = %s; want %s", r.CommitHex(), pullEnv.commit1)
		}
		if r.RefName() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.RefName())
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "FETCH_HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pullEnv.commit1 {
			t.Errorf("FETCH_HEAD = %s (first commit); want %s", r.CommitHex(), pullEnv.commit2)
		} else if r.CommitHex() != pullEnv.commit2 {
			t.Errorf("FETCH_HEAD = %s; want %s", r.CommitHex(), pullEnv.commit2)
		}
	}
}

func TestPullUpdate(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	pullEnv, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.gg(ctx, pullEnv.repoB, "pull", "-u"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pullEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pullEnv.commit1 {
			t.Errorf("HEAD = %s (first commit); want %s", r.CommitHex(), pullEnv.commit1)
		} else if r.CommitHex() != pullEnv.commit2 {
			t.Errorf("HEAD = %s; want %s", r.CommitHex(), pullEnv.commit1)
		}
		if r.RefName() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.RefName())
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "origin/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pullEnv.commit1 {
			t.Errorf("origin/master = %s (first commit); want %s", r.CommitHex(), pullEnv.commit2)
		} else if r.CommitHex() != pullEnv.commit2 {
			t.Errorf("origin/master = %s; want %s", r.CommitHex(), pullEnv.commit2)
		}
	}
}

type pullEnv struct {
	repoA, repoB     string
	commit1, commit2 string
}

func setupPullTest(ctx context.Context, env *testEnv) (*pullEnv, error) {
	repoA := filepath.Join(env.root, "repoA")
	if err := env.git.Run(ctx, "init", repoA); err != nil {
		return nil, err
	}
	gitA := env.git.WithDir(repoA)
	const fileName = "foo.txt"
	err := ioutil.WriteFile(
		filepath.Join(repoA, fileName),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		return nil, err
	}
	if err := gitA.Run(ctx, "add", fileName); err != nil {
		return nil, err
	}
	if err := gitA.Run(ctx, "commit", "-m", "initial commit"); err != nil {
		return nil, err
	}
	commit1, err := gittool.ParseRev(ctx, gitA, "HEAD")
	if err != nil {
		return nil, err
	}
	repoB := filepath.Join(env.root, "repoB")
	if err := env.git.Run(ctx, "clone", repoA, repoB); err != nil {
		return nil, err
	}
	err = ioutil.WriteFile(
		filepath.Join(repoA, fileName),
		[]byte("Hello, World!\nI learned some things...\n"),
		0666)
	if err != nil {
		return nil, err
	}
	if err := gitA.Run(ctx, "commit", "-a", "-m", "second commit"); err != nil {
		return nil, err
	}
	commit2, err := gittool.ParseRev(ctx, gitA, "HEAD")
	if err != nil {
		return nil, err
	}
	return &pullEnv{
		repoA:   repoA,
		repoB:   repoB,
		commit1: commit1.CommitHex(),
		commit2: commit2.CommitHex(),
	}, nil
}