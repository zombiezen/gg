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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
)

const commitSynopsis = "commit the specified files or all outstanding changes"

func commit(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg commit [--amend] [-m MSG] [FILE [...]]", commitSynopsis+`

aliases: ci

	Commits changes to the given files into the repository. If no files
	are given, then all changes reported by `+"`gg status`"+` will be
	committed.

	Unlike Git, gg does not require you to stage your changes into the
	index. This approximates the behavior of `+"`git commit -a`"+`, but
	this command will only change the index if the commit succeeds.`)
	amend := f.Bool("amend", false, "amend the parent of the working directory")
	msg := f.String("m", "", "use text as commit `message`")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}

	// Get status on files. First level of assurance is to stop empty commits.
	// This status info may get used for interactive commit message template.
	var pathspecs []git.Pathspec
	for _, arg := range f.Args() {
		pathspecs = append(pathspecs, git.LiteralPath(arg))
	}
	status, err := cc.git.Status(ctx, git.StatusOptions{
		Pathspecs: pathspecs,
	})
	if err != nil {
		return err
	}
	hasChanges, err := verifyNoMissingOrUnmerged(status)
	if err != nil {
		return err
	}
	if !hasChanges && !*amend {
		return errors.New("nothing changed")
	}
	var diffStatus []git.DiffStatusEntry
	if *amend {
		var err error
		diffStatus, err = amendedDiffStatus(ctx, cc.git, pathspecs)
		if err != nil {
			return err
		}
		if len(diffStatus) == 0 {
			return errors.New("amend would create an empty commit")
		}
	} else {
		// For commits, we can reuse the information from the status call.
		for _, ent := range status {
			diffStatus = append(diffStatus, statusIntoHeadDiffStatus(ent))
		}
	}

	// Get message from user.
	if *msg == "" {
		sort.Slice(diffStatus, func(i, j int) bool {
			return diffStatus[i].Name < diffStatus[j].Name
		})

		// Open message in editor.
		cfg, err := cc.git.ReadConfig(ctx)
		if err != nil {
			return err
		}
		commentChar, err := cfg.CommentChar()
		if err != nil {
			return err
		}
		initial, err := commitMessageTemplate(ctx, cc.git, diffStatus, *amend, commentChar)
		if err != nil {
			return err
		}
		editorOut, err := cc.editor.open(ctx, "COMMIT_MSG", initial)
		if err != nil {
			return err
		}
		*msg = cleanupMessage(string(editorOut), commentChar)
	} else {
		*msg = cleanupMessage(*msg, "")
	}

	// Commit or amend as appropriate.
	if len(pathspecs) > 0 {
		if *amend {
			return cc.git.AmendFiles(ctx, pathspecs, git.AmendOptions{Message: *msg})
		}
		return cc.git.CommitFiles(ctx, *msg, pathspecs, git.CommitOptions{})
	}
	if *amend {
		return cc.git.AmendAll(ctx, git.AmendOptions{Message: *msg})
	}
	return cc.git.CommitAll(ctx, *msg, git.CommitOptions{})
}

func amendedDiffStatus(ctx context.Context, g *git.Git, pathspecs []git.Pathspec) ([]git.DiffStatusEntry, error) {
	if len(pathspecs) == 0 {
		// Simple case: just run diff status.
		return g.DiffStatus(ctx, git.DiffStatusOptions{Commit1: "HEAD~"})
	}
	// More complex case: have to merge changed file status into base status.
	base, err := g.DiffStatus(ctx, git.DiffStatusOptions{Commit1: "HEAD~", Commit2: "HEAD"})
	if err != nil {
		return nil, err
	}
	// TODO(someday): If we evaluated pathspecs in-process, this DiffStatus would be unnecessary.
	filterBase, err := g.DiffStatus(ctx, git.DiffStatusOptions{Commit1: "HEAD~", Commit2: "HEAD", Pathspecs: pathspecs})
	if err != nil {
		return nil, err
	}
	local, err := g.DiffStatus(ctx, git.DiffStatusOptions{Commit1: "HEAD~", Pathspecs: pathspecs})
	if err != nil {
		return nil, err
	}

	// Remove any no-longer-modified files from base.
	unmodifiedFiles := make(map[git.TopPath]struct{})
	for _, ent := range filterBase {
		unmodifiedFiles[ent.Name] = struct{}{}
	}
	for _, ent := range local {
		delete(unmodifiedFiles, ent.Name)
	}
	status := base[:0]
	for _, ent := range base {
		if _, unmodified := unmodifiedFiles[ent.Name]; !unmodified {
			status = append(status, ent)
		}
	}

	// Use local as canonical entry.
	localMap := make(map[git.TopPath]*git.DiffStatusEntry, len(local))
	for i := range local {
		localMap[local[i].Name] = &local[i]
	}
	for i := range status {
		name := status[i].Name
		if ent := localMap[name]; ent != nil {
			status[i] = *ent
			delete(localMap, name)
		}
	}
	for _, ent := range localMap {
		status = append(status, *ent)
	}
	return status, nil
}

func commitMessageTemplate(ctx context.Context, g *git.Git, status []git.DiffStatusEntry, amend bool, commentChar string) ([]byte, error) {
	head, err := g.Head(ctx)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	if amend {
		// Use previous commit message.
		info, err := g.CommitInfo(ctx, "HEAD")
		if err != nil {
			return nil, fmt.Errorf("gather commit message template: %v", err)
		}
		buf.WriteString(info.Message)
	} else if gitDir, err := g.GitDir(ctx); err == nil {
		// Opportunistically grab the merge message.
		if mergeMsg, err := ioutil.ReadFile(filepath.Join(gitDir, "MERGE_MSG")); err == nil {
			buf.Write(mergeMsg)
		}
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n') // blank line
	buf.WriteString(commentChar)
	buf.WriteString(" Please enter a commit message.\n")
	buf.WriteString(commentChar)
	buf.WriteString(" Lines starting with '")
	buf.WriteString(commentChar)
	buf.WriteString("' will be ignored.\n")

	// Add branch info.
	buf.WriteString(commentChar)
	buf.WriteByte('\n')
	buf.WriteString(commentChar)
	buf.WriteByte(' ')
	if head.Ref == git.Head {
		buf.WriteString("detached HEAD")
	} else if b := head.Ref.Branch(); b != "" {
		buf.WriteString("branch ")
		buf.WriteString(b)
	} else {
		buf.WriteString(head.Ref.String())
	}
	buf.WriteByte('\n')

	// Add files to be committed.
	status = append([]git.DiffStatusEntry(nil), status...)
	sort.Slice(status, func(i, j int) bool {
		return status[i].Name < status[j].Name
	})
	for _, ent := range status {
		switch ent.Code {
		case git.DiffStatusAdded:
			fmt.Fprintf(buf, "%s added %s\n", commentChar, ent.Name)
		case git.DiffStatusCopied:
			fmt.Fprintf(buf, "%s copied %s\n", commentChar, ent.Name)
		case git.DiffStatusDeleted:
			fmt.Fprintf(buf, "%s removed %s\n", commentChar, ent.Name)
		case git.DiffStatusModified:
			fmt.Fprintf(buf, "%s modified %s\n", commentChar, ent.Name)
		case git.DiffStatusRenamed:
			fmt.Fprintf(buf, "%s renamed %s\n", commentChar, ent.Name)
		case git.DiffStatusChangedMode:
			fmt.Fprintf(buf, "%s chmod %s\n", commentChar, ent.Name)
		}
	}
	return buf.Bytes(), nil
}

func cleanupMessage(s string, commentPrefix string) string {
	lines := strings.SplitAfter(s, "\n")

	// Filter out comment lines and strip trailing whitespace.
	n := len(lines)
	lines = lines[:0]
	for _, line := range lines[:n] {
		if commentPrefix != "" && strings.HasPrefix(line, commentPrefix) {
			continue
		}
		lines = append(lines, strings.TrimRightFunc(line, unicode.IsSpace))
	}

	// Remove trailing blank lines.
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] != "" {
			break
		}
		lines = lines[:i]
	}

	// Join lines into single string.
	sb := new(strings.Builder)
	for _, line := range lines {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// statusIntoHeadDiffStatus converts a status entry to a diff status
// entry as if Commit1 was "HEAD".
func statusIntoHeadDiffStatus(ent git.StatusEntry) git.DiffStatusEntry {
	diffEnt := git.DiffStatusEntry{
		Name: ent.Name,
		Code: git.DiffStatusUnknown,
	}
	switch {
	case ent.Code.IsAdded():
		diffEnt.Code = git.DiffStatusAdded
	case ent.Code.IsRemoved():
		diffEnt.Code = git.DiffStatusDeleted
	case ent.Code.IsModified():
		diffEnt.Code = git.DiffStatusModified
	case ent.Code.IsRenamed():
		diffEnt.Code = git.DiffStatusRenamed
	case ent.Code.IsCopied():
		diffEnt.Code = git.DiffStatusCopied
	case ent.Code.IsUnmerged():
		diffEnt.Code = git.DiffStatusUnmerged
	}
	return diffEnt
}

func verifyNoMissingOrUnmerged(status []git.StatusEntry) (hasChanges bool, _ error) {
	missing, missingStaged, unmerged := 0, 0, 0
	for _, ent := range status {
		switch {
		case ent.Code.IsMissing():
			missing++
			if ent.Code[0] != ' ' {
				missingStaged++
			}
		case ent.Code.IsAdded() || ent.Code.IsModified() || ent.Code.IsRemoved() || ent.Code.IsCopied() || ent.Code.IsRenamed():
			hasChanges = true
		case ent.Code.IsUntracked():
			// Skip
		case ent.Code.IsUnmerged():
			unmerged++
		default:
			return false, fmt.Errorf("unhandled status: %v", ent)
		}
	}
	if unmerged == 1 {
		return false, errors.New("1 unmerged file; see 'gg status'")
	}
	if unmerged > 1 {
		return false, fmt.Errorf("%d unmerged files; see 'gg status'", unmerged)
	}
	if !hasChanges {
		switch missing {
		case 0:
			return false, nil
		case 1:
			return false, errors.New("nothing changed (1 missing file; see 'gg status')")
		default:
			return false, fmt.Errorf("nothing changed (%d missing files; see 'gg status')", missing)
		}
	}
	if missingStaged == 1 {
		return true, errors.New("git has staged changes for 1 missing file; see 'gg status'")
	}
	if missingStaged > 1 {
		return true, fmt.Errorf("git has staged changes for %d missing files; see 'gg status'", missingStaged)
	}
	return true, nil
}
