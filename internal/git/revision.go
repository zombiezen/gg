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

package git

import (
	"context"
	"fmt"
	"strings"
)

// Rev is a parsed reference to a single commit.
type Rev struct {
	commit  Hash
	refname Ref
}

// Head returns the working copy's branch revision.
func (g *Git) Head(ctx context.Context) (*Rev, error) {
	return g.ParseRev(ctx, Head.String())
}

// ParseRev parses a revision.
func (g *Git) ParseRev(ctx context.Context, refspec string) (*Rev, error) {
	if strings.HasPrefix(refspec, "-") {
		return nil, fmt.Errorf("parse revision %q: cannot start with '-'", refspec)
	}

	commitHex, err := g.RunOneLiner(ctx, '\n', "rev-parse", "-q", "--verify", "--revs-only", refspec)
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}
	h, err := ParseHash(string(commitHex))
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}

	refname, err := g.RunOneLiner(ctx, '\n', "rev-parse", "-q", "--verify", "--revs-only", "--symbolic-full-name", refspec)
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}
	return &Rev{
		commit:  h,
		refname: Ref(refname),
	}, nil
}

// Commit returns the commit hash.
func (r *Rev) Commit() Hash {
	return r.commit
}

// Ref returns the full refname or empty if r is not a symbolic revision.
func (r *Rev) Ref() Ref {
	return r.refname
}

// String returns the shortest symbolic name if possible, falling back
// to the commit hash.
func (r *Rev) String() string {
	if b := r.refname.Branch(); b != "" {
		return b
	}
	if r.refname.IsValid() {
		return r.refname.String()
	}
	return r.commit.String()
}