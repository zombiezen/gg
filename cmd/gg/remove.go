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

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
)

const removeSynopsis = "remove the specified files on the next commit"

func remove(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg remove [-f] [-r] [-after] FILE [...]", removeSynopsis+"\n\n"+
		"aliases: rm")
	after := f.Bool("after", false, "record delete for missing files")
	force := f.Bool("f", false, "forget added files, delete modified files")
	f.Alias("f", "force")
	recursive := f.Bool("r", false, "remove files under any directory specified")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 {
		return usagef("must pass one or more files to remove")
	}
	if !*after {
		if err := verifyPresent(ctx, cc.git, f.Args()); err != nil {
			return err
		}
	}
	pathspecs := make([]git.Pathspec, 0, f.NArg())
	for _, arg := range f.Args() {
		pathspecs = append(pathspecs, git.LiteralPath(arg))
	}
	return cc.git.Remove(ctx, pathspecs, git.RemoveOptions{
		Recursive: *recursive,
		Modified:  *force,
	})
}

func verifyPresent(ctx context.Context, g *git.Git, args []string) error {
	statusArgs := make([]git.Pathspec, len(args))
	for i := range args {
		statusArgs[i] = git.LiteralPath(args[i])
	}
	st, err := g.Status(ctx, git.StatusOptions{
		Pathspecs: statusArgs,
	})
	if err != nil {
		return err
	}
	for _, ent := range st {
		if ent.Code.IsMissing() {
			return fmt.Errorf("missing %s", ent.Name)
		}
	}
	return nil
}
