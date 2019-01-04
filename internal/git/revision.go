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
	"encoding/hex"
	"fmt"
	"strings"
)

// hashSize is the number of bytes in a hash.
const hashSize = 20

// A Hash is the SHA-1 hash of a Git object.
type Hash [hashSize]byte

// ParseHash parses a hex-encoded hash.
func ParseHash(s string) (Hash, error) {
	if len(s) != hex.EncodedLen(hashSize) {
		return Hash{}, fmt.Errorf("parse hash %q: wrong size", s)
	}
	var h Hash
	if _, err := hex.Decode(h[:], []byte(s)); err != nil {
		return Hash{}, fmt.Errorf("parse hash %q: %v", s, err)
	}
	return h, nil
}

// String returns the hex-encoded hash.
func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// Short returns the first 4 hex-encoded bytes of the hash.
func (h Hash) Short() string {
	return hex.EncodeToString(h[:4])
}

// A Ref is a Git reference to a commit.
type Ref string

// Top-level refs.
const (
	// Head names the commit on which the changes in the working tree
	// are based.
	Head Ref = "HEAD"

	// FetchHead records the branch which was fetched from a remote
	// repository with the last git fetch invocation.
	FetchHead Ref = "FETCH_HEAD"
)

// BranchRef returns a ref for the given branch name.
func BranchRef(b string) Ref {
	return branchPrefix + Ref(b)
}

// IsValid reports whether r is a valid reference.
func (r Ref) IsValid() bool {
	return r != "" && r[0] != '-'
}

// String returns the ref as a string.
func (r Ref) String() string {
	return string(r)
}

// IsBranch reports whether r starts with "refs/heads/".
func (r Ref) IsBranch() bool {
	return strings.HasPrefix(string(r), branchPrefix)
}

// Branch returns the string after "refs/heads/" or an empty string
// if the ref does not start with "refs/heads/".
func (r Ref) Branch() string {
	if !r.IsBranch() {
		return ""
	}
	return string(r[len(branchPrefix):])
}

// IsTag reports whether r starts with "refs/tags/".
func (r Ref) IsTag() bool {
	return strings.HasPrefix(string(r), tagPrefix)
}

// Tag returns the string after "refs/tags/" or an empty string
// if the ref does not start with "refs/tags/".
func (r Ref) Tag() string {
	if !r.IsTag() {
		return ""
	}
	return string(r[len(tagPrefix):])
}

// Ref prefixes.
const (
	branchPrefix = "refs/heads/"
	tagPrefix    = "refs/tags/"
)

// Head returns the working copy's branch revision.
func (g *Git) Head(ctx context.Context) (*Rev, error) {
	return g.ParseRev(ctx, Head.String())
}

// ParseRev parses a revision.
func (g *Git) ParseRev(ctx context.Context, refspec string) (*Rev, error) {
	errPrefix := fmt.Sprintf("parse revision %q", refspec)
	if err := validateRev(refspec); err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}

	out, err := g.run(ctx, errPrefix, []string{g.exe, "rev-parse", "-q", "--verify", "--revs-only", refspec})
	if err != nil {
		return nil, err
	}
	commitHex, err := oneLine(out)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	h, err := ParseHash(commitHex)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}

	out, err = g.run(ctx, errPrefix, []string{g.exe, "rev-parse", "-q", "--verify", "--revs-only", "--symbolic-full-name", refspec})
	if err != nil {
		return nil, err
	}
	if out == "" {
		// No associated ref name, but is a valid commit.
		return &Rev{Commit: h}, nil
	}
	refName, err := oneLine(out)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	return &Rev{
		Commit: h,
		Ref:    Ref(refName),
	}, nil
}

// ListRefs lists all of the refs in the repository.
func (g *Git) ListRefs(ctx context.Context) (map[Ref]Hash, error) {
	const errPrefix = "git show-ref"
	out, err := g.run(ctx, errPrefix, []string{g.exe, "show-ref", "--dereference"})
	if err != nil {
		return nil, err
	}
	refs := make(map[Ref]Hash)
	tags := make(map[Ref]bool)
	for len(out) > 0 {
		eol := strings.IndexByte(out, '\n')
		if eol == -1 {
			return refs, fmt.Errorf("%s: unexpected EOF", errPrefix)
		}
		line := out[:eol]
		out = out[eol+1:]

		sp := strings.IndexByte(line, ' ')
		if sp == -1 {
			return refs, fmt.Errorf("%s: could not parse line %q", errPrefix, line)
		}
		h, err := ParseHash(line[:sp])
		if err != nil {
			return refs, fmt.Errorf("%s: parse hash of ref %q: %v", errPrefix, line[sp+1:], err)
		}
		ref := Ref(line[sp+1:])
		if strings.HasSuffix(string(ref), "^{}") {
			// Dereferenced tag. This takes precedence over the previous hash stored in the map.
			ref = ref[:len(ref)-3]
			if tags[ref] {
				return refs, fmt.Errorf("%s: multiple hashes found for tag %v", errPrefix, ref)
			}
			tags[ref] = true
		} else if _, exists := refs[ref]; exists {
			return refs, fmt.Errorf("%s: multiple hashes found for %v", errPrefix, ref)
		}
		refs[ref] = h
	}
	return refs, nil
}

// Rev is a parsed reference to a single commit.
type Rev struct {
	Commit Hash
	Ref    Ref
}

// String returns the shortest symbolic name if possible, falling back
// to the commit hash.
func (r *Rev) String() string {
	if b := r.Ref.Branch(); b != "" {
		return b
	}
	if r.Ref.IsValid() {
		return r.Ref.String()
	}
	return r.Commit.String()
}
