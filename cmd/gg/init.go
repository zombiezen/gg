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

	"gg-scm.io/tool/internal/flag"
	"gg-scm.io/tool/internal/repodb"
)

const initSynopsis = "create a new repository in the given directory"

func init_(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg init [DEST]", initSynopsis+`

	If no directory is given, the current directory is used.`)
	useRepoDB := f.Bool("experimental-index", false, "enable experimental indexing")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("cannot pass more than one argument to init")
	}
	dst := f.Arg(0)
	if dst == "" {
		dst = "."
	}
	if err := cc.git.Init(ctx, dst); err != nil {
		return err
	}
	if !*useRepoDB {
		return nil
	}
	dir, err := cc.git.WithDir(dst).CommonDir(ctx)
	if err != nil {
		return err
	}
	db, err := repodb.Create(ctx, dir)
	if err != nil {
		return err
	}
	if err := repodb.Sync(ctx, db, dir); err != nil {
		return err
	}
	return nil
}
