// Copyright 2018 The gg Authors
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
	"fmt"
	"testing"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/escape"
	"gg-scm.io/tool/internal/filesystem"
)

func TestRebase(t *testing.T) {
	t.Parallel()
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}

		// Create repository with two commits on a branch called "topic" and
		// a diverging commit on "main".
		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		baseRev, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.git.NewBranch(ctx, "topic", git.BranchOptions{Track: true}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("mainline.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "mainline.txt"); err != nil {
			t.Fatal(err)
		}
		head, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		if err := env.git.CheckoutBranch(ctx, "topic", git.CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "foo.txt"); err != nil {
			t.Fatal(err)
		}
		c1, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "bar.txt"); err != nil {
			t.Fatal(err)
		}
		c2, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		names := map[git.Hash]string{
			baseRev.Commit: "initial import",
			c1:             "change 1",
			c2:             "change 2",
			head:           "mainline change",
		}

		// Call gg with the rebase arguments to move onto main.
		ggArgs := []string{"rebase"}
		if arg := argFunc(head); arg != "" {
			ggArgs = append(ggArgs, "-base="+arg, "-dst="+arg)
		}
		_, err = env.gg(ctx, env.root.String(), ggArgs...)
		if err != nil {
			t.Error(err)
		}

		curr, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// Verify that HEAD points to a new commit.
		if _, existedBefore := names[curr.Commit]; existedBefore {
			t.Fatalf("rebase HEAD = %s; want new commit", prettyCommit(curr.Commit, names))
		}
		// Verify that HEAD is on the topic branch.
		if want := git.Ref("refs/heads/topic"); curr.Ref != want {
			t.Errorf("rebase changed ref to %s; want %s", curr.Ref, want)
		}
		// Verify that HEAD contains all the files.
		if err := objectExists(ctx, env.git, curr.Commit.String(), "foo.txt"); err != nil {
			t.Error("foo.txt not in second rebased change:", err)
		}
		if err := objectExists(ctx, env.git, curr.Commit.String(), "bar.txt"); err != nil {
			t.Error("bar.txt not in second rebased change:", err)
		}
		if err := objectExists(ctx, env.git, curr.Commit.String(), "mainline.txt"); err != nil {
			t.Error("mainline.txt not in second rebased change:", err)
		}

		parent, err := env.git.ParseRev(ctx, "HEAD~1")
		if err != nil {
			t.Fatal(err)
		}
		// Verify that the parent is a new commit.
		if _, existedBefore := names[parent.Commit]; existedBefore {
			t.Fatalf("rebase HEAD~1 = %s; want new commit", prettyCommit(parent.Commit, names))
		}
		// Verify that HEAD~1 contains all the files except the one in the second change.
		if err := objectExists(ctx, env.git, parent.Commit.String(), "foo.txt"); err != nil {
			t.Error("foo.txt not in first rebased change:", err)
		}
		if err := objectExists(ctx, env.git, parent.Commit.String(), "mainline.txt"); err != nil {
			t.Error("mainline.txt not in first rebased change:", err)
		}
		if err := objectExists(ctx, env.git, parent.Commit.String(), "bar.txt"); err == nil {
			t.Error("bar.txt in first rebased change")
		}

		// Verify that the grandparent is the diverged upstream commit.
		grandparent, err := env.git.ParseRev(ctx, "HEAD~2")
		if err != nil {
			t.Fatal(err)
		}
		if grandparent.Commit != head {
			t.Errorf("HEAD~2 = %s; want %s", prettyCommit(grandparent.Commit, names), prettyCommit(head, names))
		}
	})
}

func TestRebase_Src(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with two commits on a branch called "topic" and
	// a diverging commit on "main".
	if err := env.initRepoWithHistory(ctx, "."); err != nil {
		t.Fatal(err)
	}
	baseRev, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.NewBranch(ctx, "topic", git.BranchOptions{Track: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("mainline.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "mainline.txt"); err != nil {
		t.Fatal(err)
	}
	head, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.CheckoutBranch(ctx, "topic", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	c1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "bar.txt"); err != nil {
		t.Fatal(err)
	}
	c2, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	names := map[git.Hash]string{
		baseRev.Commit: "initial import",
		c1:             "change 1",
		c2:             "change 2",
		head:           "mainline change",
	}

	// Call gg to rebase just the second change onto its upstream branch (main).
	if _, err := env.gg(ctx, env.root.String(), "rebase", "-src="+c2.String()); err != nil {
		t.Error(err)
	}

	curr, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Verify that HEAD points to a new commit.
	if _, existedBefore := names[curr.Commit]; existedBefore {
		t.Fatalf("rebase HEAD = %s; want new commit", prettyCommit(curr.Commit, names))
	}
	// Verify that HEAD is on the topic branch.
	if want := git.Ref("refs/heads/topic"); curr.Ref != want {
		t.Errorf("rebase changed ref to %s; want %s", curr.Ref, want)
	}
	// Verify that HEAD contains all the files except the first topic change.
	if err := objectExists(ctx, env.git, curr.Commit.String(), "foo.txt"); err == nil {
		t.Error("foo.txt is in rebased change")
	}
	if err := objectExists(ctx, env.git, curr.Commit.String(), "bar.txt"); err != nil {
		t.Error("bar.txt not in rebased change:", err)
	}
	if err := objectExists(ctx, env.git, curr.Commit.String(), "mainline.txt"); err != nil {
		t.Error("mainline.txt not in rebased change:", err)
	}

	// Verify that the parent commit is the diverged main commit.
	parent, err := env.git.ParseRev(ctx, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.Commit != head {
		t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.Commit, names), prettyCommit(head, names))
	}
}

func TestRebase_SrcUnrelated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with two commits on a branch called "topic".
	if err := env.initRepoWithHistory(ctx, "."); err != nil {
		t.Fatal(err)
	}
	baseRev, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.NewBranch(ctx, "topic", git.BranchOptions{Checkout: true, Track: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	c1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "bar.txt"); err != nil {
		t.Fatal(err)
	}
	c2, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	names := map[git.Hash]string{
		baseRev.Commit: "initial import",
		c1:             "change 1",
		c2:             "change 2",
	}

	// Call gg on main to rebase the second commit onto main.
	if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, env.root.String(), "rebase", "-src="+c2.String(), "-dst=HEAD"); err != nil {
		t.Error(err)
	}

	curr, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Verify that HEAD points to a new commit.
	if _, existedBefore := names[curr.Commit]; existedBefore {
		t.Fatalf("rebase HEAD = %s; want new commit", prettyCommit(curr.Commit, names))
	}
	// Verify that HEAD is on the main branch.
	if want := git.Ref("refs/heads/main"); curr.Ref != want {
		t.Errorf("rebase changed ref to %s; want %s", curr.Ref, want)
	}
	// Verify that HEAD contains the file from the second change but not from the first change.
	if err := objectExists(ctx, env.git, curr.Commit.String(), "foo.txt"); err == nil {
		t.Error("foo.txt in rebased change")
	}
	if err := objectExists(ctx, env.git, curr.Commit.String(), "bar.txt"); err != nil {
		t.Error("bar.txt not in rebased change:", err)
	}

	// Verify that the parent is the initial commit.
	parent, err := env.git.ParseRev(ctx, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.Commit != baseRev.Commit {
		t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.Commit, names), prettyCommit(baseRev.Commit, names))
	}
}

func TestRebase_Base(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create a repository with this commit topology:
	//
	// *-----*  main
	//  \
	//   *-*-*  topic
	//      \
	//       *  magic
	if err := env.initRepoWithHistory(ctx, "."); err != nil {
		t.Fatal(err)
	}
	baseRev, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.NewBranch(ctx, "topic", git.BranchOptions{Track: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("mainline.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "mainline.txt"); err != nil {
		t.Fatal(err)
	}
	head, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.CheckoutBranch(ctx, "topic", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	c1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "bar.txt"); err != nil {
		t.Fatal(err)
	}
	c2, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.NewBranch(ctx, "magic", git.BranchOptions{Track: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("baz.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "baz.txt"); err != nil {
		t.Fatal(err)
	}
	c3, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.CheckoutBranch(ctx, "magic", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("shazam.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "shazam.txt"); err != nil {
		t.Fatal(err)
	}
	magic, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	names := map[git.Hash]string{
		baseRev.Commit: "initial import",
		c1:             "change 1",
		c2:             "change 2",
		c3:             "change 3",
		magic:          "magic",
		head:           "mainline change",
	}

	// Call gg on the topic branch to rebase everything past the merge
	// point of topic and magic (change 2) onto topic's upstream (main).
	if err := env.git.CheckoutBranch(ctx, "topic", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, env.root.String(), "rebase", "-base="+magic.String()); err != nil {
		t.Error(err)
	}

	curr, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Verify that HEAD points to a new commit.
	if _, existedBefore := names[curr.Commit]; existedBefore {
		t.Fatalf("rebase HEAD = %s; want new commit", prettyCommit(curr.Commit, names))
	}
	// Verify that HEAD is on the topic branch.
	if want := git.Ref("refs/heads/topic"); curr.Ref != want {
		t.Errorf("rebase changed ref to %s; want %s", curr.Ref, want)
	}
	// Verify that HEAD contains the mainline file and the change 3 file, but no others.
	if err := objectExists(ctx, env.git, curr.Commit.String(), "foo.txt"); err == nil {
		t.Error("foo.txt in rebased change")
	}
	if err := objectExists(ctx, env.git, curr.Commit.String(), "bar.txt"); err == nil {
		t.Error("bar.txt in rebased change")
	}
	if err := objectExists(ctx, env.git, curr.Commit.String(), "baz.txt"); err != nil {
		t.Error("baz.txt not in rebased change:", err)
	}
	if err := objectExists(ctx, env.git, curr.Commit.String(), "mainline.txt"); err != nil {
		t.Error("mainline.txt not in rebased change:", err)
	}
	if err := objectExists(ctx, env.git, curr.Commit.String(), "shazam.txt"); err == nil {
		t.Error("shazam.txt in rebased change")
	}

	// Verify that the parent commit is the diverged upstream commit.
	parent, err := env.git.ParseRev(ctx, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.Commit != head {
		t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.Commit, names), prettyCommit(head, names))
	}
}

func TestRebase_ResetUpstream(t *testing.T) {
	// Regression test for https://github.com/gg-scm/gg/issues/41

	t.Parallel()
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}

		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		baseRev, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// Create a commit on main.
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "foo.txt"); err != nil {
			t.Fatal(err)
		}
		feature, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		// Create topic branch with the new commit.
		if err := env.git.NewBranch(ctx, "topic", git.BranchOptions{Track: true}); err != nil {
			t.Fatal(err)
		}
		// Move main branch back to the base commit.
		// Importantly, this will be recorded in the reflog.
		if err := env.git.Run(ctx, "reset", "--hard", baseRev.Commit.String()); err != nil {
			t.Fatal(err)
		}
		// Create a new commit on main.
		if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "bar.txt"); err != nil {
			t.Fatal(err)
		}
		upstream, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		names := map[git.Hash]string{
			baseRev.Commit: "initial import",
			feature:        "feature change",
			upstream:       "upstream change",
		}

		// Call gg on the topic branch to rebase all changes past the merge
		// point of main and topic (the base revision) on top of the new
		// main commit.
		if err := env.git.CheckoutBranch(ctx, "topic", git.CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}
		rebaseArgs := []string{"rebase", "-dst=main"}
		if arg := argFunc(upstream); arg != "" {
			rebaseArgs = append(rebaseArgs, "-base="+arg)
		}
		if _, err := env.gg(ctx, env.root.String(), rebaseArgs...); err != nil {
			t.Error(err)
		}

		curr, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// Verify that HEAD points to a new commit.
		if _, existedBefore := names[curr.Commit]; existedBefore {
			t.Fatalf("rebase HEAD = %s; want new commit", prettyCommit(curr.Commit, names))
		}
		// Verify that HEAD is on the topic branch.
		if want := git.Ref("refs/heads/topic"); curr.Ref != want {
			t.Errorf("rebase changed ref to %s; want %s", curr.Ref, want)
		}
		// Verify that HEAD contains both of the files.
		if err := objectExists(ctx, env.git, curr.Commit.String(), "foo.txt"); err != nil {
			t.Error("foo.txt not in rebased change:", err)
		}
		if err := objectExists(ctx, env.git, curr.Commit.String(), "bar.txt"); err != nil {
			t.Error("bar.txt not in rebased change:", err)
		}
		// Verify that the parent commit is the diverged upstream commit.
		parent, err := env.git.ParseRev(ctx, "HEAD~")
		if err != nil {
			t.Fatal(err)
		}
		if parent.Commit != upstream {
			t.Errorf("HEAD~ = %s; want %s", prettyCommit(parent.Commit, names), prettyCommit(upstream, names))
		}
	})
}

func TestRebase_NoUpstream(t *testing.T) {
	// Regression test for https://github.com/gg-scm/gg/issues/127

	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	if err := env.initRepoWithHistory(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Call gg rebase.
	stdout, err := env.gg(ctx, env.root.String(), "rebase")
	if err == nil {
		t.Error("gg succeeded even though it should have returned an error")
	} else {
		t.Log(err)
		t.Logf("%s", stdout)
	}
}

func TestHistedit(t *testing.T) {
	t.Parallel()
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}

		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		baseRev, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// Create a new branch.
		if err := env.git.NewBranch(ctx, "foo", git.BranchOptions{Track: true}); err != nil {
			t.Fatal(err)
		}
		// Create a commit on main.
		if err := env.root.Apply(filesystem.Write("upstream.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "upstream.txt"); err != nil {
			t.Fatal(err)
		}
		head, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		// Check out foo and create a commit.
		if err := env.git.CheckoutBranch(ctx, "foo", git.CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "foo.txt"); err != nil {
			t.Fatal(err)
		}
		c, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		names := map[git.Hash]string{
			baseRev.Commit: "initial import",
			c:              "branch change",
			head:           "mainline change",
		}

		// Call gg histedit on foo branch.
		rebaseEditor, err := env.editorCmd([]byte("reword " + c.String() + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		const wantMessage = "New commit message for foo.txt\n"
		msgEditor, err := env.editorCmd([]byte(wantMessage))
		if err != nil {
			t.Fatal(err)
		}
		config := fmt.Sprintf("[sequence]\neditor = %s\n[core]\neditor = %s\n",
			escape.GitConfig(rebaseEditor), escape.GitConfig(msgEditor))
		if err := env.writeConfig([]byte(config)); err != nil {
			t.Fatal(err)
		}
		out, err := env.gg(ctx, env.root.String(), appendNonEmpty([]string{"histedit"}, argFunc(head))...)
		if err != nil {
			t.Fatalf("failed: %v; output:\n%s", err, out)
		}

		curr, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// Verify that HEAD points to a new commit.
		if _, existedBefore := names[curr.Commit]; existedBefore {
			t.Fatalf("rebase HEAD = %s; want new commit", prettyCommit(curr.Commit, names))
		}
		// Verify that HEAD is on the foo branch.
		if want := git.Ref("refs/heads/foo"); curr.Ref != want {
			t.Errorf("rebase changed ref to %s; want %s", curr.Ref, want)
		}
		// Verify that HEAD contains foo.txt but not upstream.txt.
		if err := objectExists(ctx, env.git, curr.Commit.String(), "foo.txt"); err != nil {
			t.Error("foo.txt not in rebased change:", err)
		}
		if err := objectExists(ctx, env.git, curr.Commit.String(), "upstream.txt"); err == nil {
			t.Error("upstream.txt in rebased change")
		}
		// Verify that the commit message matches what was given.
		if info, err := env.git.CommitInfo(ctx, curr.Commit.String()); err != nil {
			t.Error(err)
		} else if info.Message != wantMessage {
			t.Errorf("commit message = %q; want %q", info.Message, wantMessage)
		}

		// Verify that the parent commit is the base commit.
		parent, err := env.git.ParseRev(ctx, "HEAD~1")
		if err != nil {
			t.Fatal(err)
		}
		if parent.Commit != baseRev.Commit {
			t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.Commit, names), prettyCommit(baseRev.Commit, names))
		}
	})
}

func TestHistedit_ContinueWithModifications(t *testing.T) {
	t.Parallel()
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}

		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		baseRev, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// Create a new branch.
		if err := env.git.NewBranch(ctx, "foo", git.BranchOptions{Track: true}); err != nil {
			t.Fatal(err)
		}
		// Create a commit on main.
		if err := env.root.Apply(filesystem.Write("upstream.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "upstream.txt"); err != nil {
			t.Fatal(err)
		}
		head, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		// Create two commits on foo.
		if err := env.git.CheckoutBranch(ctx, "foo", git.CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", "This is the original data\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "foo.txt"); err != nil {
			t.Fatal(err)
		}
		if err := env.git.Commit(ctx, "Divergence 1", git.CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		rev1, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "bar.txt"); err != nil {
			t.Fatal(err)
		}
		const wantMessage2 = "Divergence 2\n"
		if err := env.git.Commit(ctx, wantMessage2, git.CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		rev2, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		names := map[git.Hash]string{
			baseRev.Commit: "initial import",
			rev1.Commit:    "branch change 1",
			rev2.Commit:    "branch change 2",
			head:           "mainline change",
		}

		// Call gg histedit on foo branch.
		rebaseEditor, err := env.editorCmd([]byte(
			"edit " + rev1.Commit.String() + "\n" +
				"pick " + rev2.Commit.String() + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		const wantMessage1 = "New commit message for foo.txt\n"
		msgEditor, err := env.editorCmd([]byte(wantMessage1))
		if err != nil {
			t.Fatal(err)
		}
		config := fmt.Sprintf("[sequence]\neditor = %s\n[core]\neditor = %s\n",
			escape.GitConfig(rebaseEditor), escape.GitConfig(msgEditor))
		if err := env.writeConfig([]byte(config)); err != nil {
			t.Fatal(err)
		}
		out, err := env.gg(ctx, env.root.String(), appendNonEmpty([]string{"histedit"}, argFunc(head))...)
		if err != nil {
			t.Fatalf("failed: %v; output:\n%s", err, out)
		}

		// Stopped for amending after applying the first commit.
		// Verify that the parent commit is the base commit.
		parent, err := env.git.ParseRev(ctx, "HEAD~")
		if err != nil {
			t.Fatal(err)
		}
		if parent.Commit != baseRev.Commit {
			t.Errorf("After first stop, HEAD~ = %s; want %s",
				prettyCommit(parent.Commit, names),
				prettyCommit(baseRev.Commit, names))
		}
		const amendedData = "This is edited history\n"
		err = env.root.Apply(
			// Write new data to foo.txt.
			filesystem.Write("foo.txt", amendedData),
			// Write content to an untracked file.
			filesystem.Write("untracked.txt", dummyContent),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Continue rebase, should be finished.
		out, err = env.gg(ctx, env.root.String(), "histedit", "-continue")
		if err != nil {
			t.Fatalf("failed: %v; output:\n%s", err, out)
		}

		// Verify that the grandparent commit is the base commit.
		grandparent, err := env.git.ParseRev(ctx, "HEAD~2")
		if err != nil {
			t.Fatal(err)
		}
		if grandparent.Commit != baseRev.Commit {
			t.Errorf("After continuing, HEAD~2 = %s; want %s",
				prettyCommit(grandparent.Commit, names),
				prettyCommit(baseRev.Commit, names))
		}

		// Verify that the commit message of the first edited commit is the message from the editor.
		if info, err := env.git.CommitInfo(ctx, "HEAD~"); err != nil {
			t.Errorf("Rebased change 1: %v", err)
		} else if info.Message != wantMessage1 {
			t.Errorf("Rebased change 1 commit message = %q; want %q", info.Message, wantMessage1)
		}
		// Verify that the content of foo.txt in the first edited commit is the rewritten content.
		if content, err := catBlob(ctx, env.git, "HEAD~", "foo.txt"); err != nil {
			t.Error(err)
		} else if string(content) != amendedData {
			t.Errorf("foo.txt @ HEAD~ = %q; want %q", content, amendedData)
		}
		// Verify that bar.txt does not exist in the first edited commit.
		if err := objectExists(ctx, env.git, "HEAD~", "bar.txt"); err == nil {
			t.Error("bar.txt @ HEAD~ exists")
		}
		// Verify that untracked.txt does not exist in the first edited commit.
		if err := objectExists(ctx, env.git, "HEAD~", "untracked.txt"); err == nil {
			t.Error("untracked.txt @ HEAD~ exists")
		}

		// Verify that the commit message of the second edited commit is the same as before.
		if info, err := env.git.CommitInfo(ctx, "HEAD"); err != nil {
			t.Errorf("Rebased change 2: %v", err)
		} else if info.Message != wantMessage2 {
			t.Errorf("Rebased change 2 commit message = %q; want %q", info.Message, wantMessage2)
		}
		// Verify that the content of foo.txt in the second edited commit is the rewritten content.
		if content, err := catBlob(ctx, env.git, "HEAD", "foo.txt"); err != nil {
			t.Error(err)
		} else if string(content) != amendedData {
			t.Errorf("foo.txt @ HEAD = %q; want %q", content, amendedData)
		}
		// Verify that bar.txt exists in the second edited commit.
		if err := objectExists(ctx, env.git, "HEAD", "bar.txt"); err != nil {
			t.Error(err)
		}
	})
}

func TestHistedit_ContinueNoModifications(t *testing.T) {
	t.Parallel()
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}

		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		baseRev, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// Create a new branch.
		if err := env.git.NewBranch(ctx, "foo", git.BranchOptions{Track: true}); err != nil {
			t.Fatal(err)
		}
		// Create a commit on main.
		if err := env.root.Apply(filesystem.Write("upstream.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "upstream.txt"); err != nil {
			t.Fatal(err)
		}
		head, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		// Create two commits on foo.
		if err := env.git.CheckoutBranch(ctx, "foo", git.CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", "This is the original data\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "foo.txt"); err != nil {
			t.Fatal(err)
		}
		const wantMessage1 = "Divergence 1\n"
		if err := env.git.Commit(ctx, wantMessage1, git.CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		rev1, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "bar.txt"); err != nil {
			t.Fatal(err)
		}
		const wantMessage2 = "Divergence 2\n"
		if err := env.git.Commit(ctx, wantMessage2, git.CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		rev2, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		names := map[git.Hash]string{
			baseRev.Commit: "initial import",
			rev1.Commit:    "branch change 1",
			rev2.Commit:    "branch change 2",
			head:           "mainline change",
		}

		// Call gg histedit on foo branch.
		rebaseEditor, err := env.editorCmd([]byte(
			"edit " + rev1.Commit.String() + "\n" +
				"pick " + rev2.Commit.String() + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		msgEditor, err := env.editorCmd([]byte("Amended message, should not occur!\n"))
		if err != nil {
			t.Fatal(err)
		}
		config := fmt.Sprintf("[sequence]\neditor = %s\n[core]\neditor = %s\n",
			escape.GitConfig(rebaseEditor), escape.GitConfig(msgEditor))
		if err := env.writeConfig([]byte(config)); err != nil {
			t.Fatal(err)
		}
		out, err := env.gg(ctx, env.root.String(), appendNonEmpty([]string{"histedit"}, argFunc(head))...)
		if err != nil {
			t.Fatalf("failed: %v; output:\n%s", err, out)
		}

		// Stopped for amending after applying the first commit.
		// Verify that the parent commit is the base commit.
		parent, err := env.git.ParseRev(ctx, "HEAD~")
		if err != nil {
			t.Fatal(err)
		}
		if parent.Commit != baseRev.Commit {
			t.Errorf("After first stop, HEAD~ = %s; want %s",
				prettyCommit(parent.Commit, names),
				prettyCommit(baseRev.Commit, names))
		}
		rebased1, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		names[rebased1.Commit] = "rebased change 1"

		// Continue rebase, should be finished.
		out, err = env.gg(ctx, env.root.String(), "histedit", "-continue")
		if err != nil {
			t.Fatalf("failed: %v; output:\n%s", err, out)
		}
		// Verify that the grandparent commit is the base commit.
		grandparent, err := env.git.ParseRev(ctx, "HEAD~2")
		if err != nil {
			t.Fatal(err)
		}
		if grandparent.Commit != baseRev.Commit {
			t.Errorf("After continuing, HEAD~2 = %s; want %s",
				prettyCommit(grandparent.Commit, names),
				prettyCommit(baseRev.Commit, names))
		}
		// Verify that the commit message of the first edited commit is the same as before.
		if info, err := env.git.CommitInfo(ctx, "HEAD~"); err != nil {
			t.Errorf("Rebased change 1: %v", err)
		} else if info.Message != wantMessage1 {
			t.Errorf("Rebased change 1 commit message = %q; want %q", info.Message, wantMessage1)
		}
		// Verify that the first edited commit hash is the same as what was
		// observed during the rebase operation.
		if r, err := env.git.ParseRev(ctx, "HEAD~"); err != nil {
			t.Errorf("Rebased change 1: %v", err)
		} else if r.Commit != rebased1.Commit {
			t.Errorf("After continuing, HEAD~ = %s; want %s",
				prettyCommit(r.Commit, names),
				prettyCommit(rebased1.Commit, names))
		}
		// Verify that the commit message of the second edited commit is the same as before.
		if info, err := env.git.CommitInfo(ctx, "HEAD"); err != nil {
			t.Errorf("Rebased change 2: %v", err)
		} else if info.Message != wantMessage2 {
			t.Errorf("Rebased change 2 commit message = %q; want %q", info.Message, wantMessage2)
		}
		// Verify that the second edited commit contains both foo.txt and bar.txt.
		if err := objectExists(ctx, env.git, "HEAD", "foo.txt"); err != nil {
			t.Error(err)
		}
		if err := objectExists(ctx, env.git, "HEAD", "bar.txt"); err != nil {
			t.Error(err)
		}
	})
}

func TestHistedit_NoUpstream(t *testing.T) {
	// Regression test for https://github.com/gg-scm/gg/issues/127

	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	if err := env.initRepoWithHistory(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Call gg histedit.
	stdout, err := env.gg(ctx, env.root.String(), "histedit")
	if err == nil {
		t.Error("gg succeeded even though it should have returned an error")
	} else {
		t.Log(err)
		t.Logf("%s", stdout)
	}
}

type rebaseArgFunc = func(mainCommit git.Hash) string

func runRebaseArgVariants(t *testing.T, f func(*testing.T, rebaseArgFunc)) {
	t.Run("NoArg", func(t *testing.T) {
		f(t, func(_ git.Hash) string {
			return ""
		})
	})
	t.Run("BranchName", func(t *testing.T) {
		f(t, func(_ git.Hash) string {
			return "main"
		})
	})
	t.Run("CommitHex", func(t *testing.T) {
		f(t, func(mainCommit git.Hash) string {
			return mainCommit.String()
		})
	})
}

func appendNonEmpty(args []string, s string) []string {
	if s == "" {
		return args
	}
	return append(args, s)
}
