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
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestAdd(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	repoPath := filepath.Join(env.root, "repo")
	if err := env.git.Run(ctx, "init", repoPath); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, "foo.txt"),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, repoPath, "add", "foo.txt"); err != nil {
		t.Error("gg:", err)
	}
	git := env.git.WithDir(repoPath)
	p, err := git.Start(ctx, "status", "--porcelain", "-z", "-unormal")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Wait()
	r := bufio.NewReader(p)
	found := false
	for {
		ent, err := readStatusEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("read status entry:", err)
		}
		if ent.name != "foo.txt" {
			t.Errorf("unknown line in status: '%c%c' %s", ent.code[0], ent.code[1], ent.name)
			continue
		}
		found = true
		if ent.code[0] != 'A' && ent.code[1] != 'A' {
			t.Errorf("status = '%c%c'; want to contain 'A'", ent.code[0], ent.code[1])
		}
	}
	if !found {
		t.Errorf("file foo.txt not in git status")
	}
}

func TestAdd_DoesNotStageModified(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	repoPath := filepath.Join(env.root, "repo")
	if err := env.git.Run(ctx, "init", repoPath); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, "foo.txt"),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	git := env.git.WithDir(repoPath)
	if err := git.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := git.Run(ctx, "commit", "-m", "commit"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, "foo.txt"),
		[]byte("Something different\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, repoPath, "add", "foo.txt"); err != nil {
		t.Error("gg:", err)
	}
	p, err := git.Start(ctx, "status", "--porcelain", "-z", "-unormal")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Wait()
	r := bufio.NewReader(p)
	found := false
	for {
		ent, err := readStatusEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("read status entry:", err)
		}
		if ent.name != "foo.txt" {
			t.Errorf("unknown line in status: '%c%c' %s", ent.code[0], ent.code[1], ent.name)
			continue
		}
		found = true
		if ent.code[0] != ' ' && ent.code[1] != 'M' {
			t.Errorf("status = '%c%c'; want ' M'", ent.code[0], ent.code[1])
		}
	}
	if !found {
		t.Errorf("file foo.txt not in git status")
	}
}
