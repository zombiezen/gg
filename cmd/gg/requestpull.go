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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/gittool"
)

const requestPullSynopsis = "create a GitHub pull request"

func requestPull(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg requestpull [-n] [BRANCH]", requestPullSynopsis+`

aliases: pr

	Create a new GitHub pull request for the given branch (defaults to the
	one currently checked out). The source will be inferred from the
	branch's remote push information and the destination will be inferred
	from upstream fetch information. This command does not push any new
	commits; it just creates a pull request.

	For non-dry runs, you must create a [personal access token][] at
	https://github.com/settings/tokens/new and save it to
	`+"`$XDG_CONFIG_HOME/gg/github_token`"+` (or in any other directory
	in `+"`$XDG_CONFIG_DIRS`"+`). By default, this would be
	`+"`~/.config/gg/github_token`"+`. gg needs at least `+"`public_repo`"+` scope
	to be able to create pull requests, but you can grant `+"`repo`"+` scope to
	create pull requests in any repositories you have access to.

[personal access token]: https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/`)
	dryRun := f.Bool("n", false, "prints the pull request instead of creating it")
	f.Alias("n", "dry-run")
	maintainerEdits := f.Bool("maintainer-edits", true, "allow maintainers to edit this branch")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("only one branch allowed")
	}
	cfg, err := gittool.ReadConfig(ctx, cc.git)
	if err != nil {
		return err
	}
	var token []byte
	if !*dryRun {
		var err error
		token, err = cc.xdgDirs.readConfig("github_token")
		if os.IsNotExist(err) {
			fmt.Fprintln(cc.stderr, "Missing github_token config file. Generate a new GitHub personal access")
			fmt.Fprintln(cc.stderr, "token at https://github.com/settings/tokens/new?scopes=repo and save it to")
			fmt.Fprintln(cc.stderr, filepath.Join(cc.xdgDirs.configPaths()[0], "gg", "github_token")+".")
			return err
		}
		if err != nil {
			return err
		}
		token = bytes.TrimSpace(token)
	}

	// Find local branch name.
	var branch string
	if branchArg := f.Arg(0); branchArg == "" {
		branch = currentBranch(ctx, cc)
		if branch == "" {
			return errors.New("no branch currently checked out")
		}
	} else {
		rev, err := gittool.ParseRev(ctx, cc.git, branchArg)
		if err != nil {
			return err
		}
		branch = rev.Ref().Branch()
		if branch == "" {
			return fmt.Errorf("%s is not a branch", branchArg)
		}
	}

	// Find base repository and ref.
	baseRemote := cfg.Value("branch." + branch + ".remote")
	if baseRemote == "" {
		remotes, _ := listRemotes(ctx, cc.git)
		if _, ok := remotes["origin"]; !ok {
			return errors.New("branch has no remote and no remote named \"origin\" found")
		}
		baseRemote = "origin"
	}
	baseURL := cfg.Value("remote." + baseRemote + ".url")
	baseOwner, baseRepo := parseGitHubRemoteURL(baseURL)
	if baseOwner == "" || baseRepo == "" {
		return fmt.Errorf("%s is not a GitHub repository", baseURL)
	}
	baseBranch := inferUpstream(cfg, branch).Branch()

	// Find head repository and ref.
	headRemote, err := inferPushRepo(ctx, cc.git, cfg, branch)
	if err != nil {
		return err
	}
	headURL := cfg.Value("remote." + headRemote + ".pushurl")
	if headURL == "" {
		headURL = cfg.Value("remote." + headRemote + ".url")
	}
	headOwner, _ := parseGitHubRemoteURL(headURL)
	if headOwner == "" {
		return fmt.Errorf("%s is not a GitHub repository", headURL)
	}

	// Create pull request.
	title, body, err := inferPullRequestMessage(ctx, cc.git, branch+"@{upstream}", branch)
	if err != nil {
		return err
	}
	if *dryRun {
		_, err := fmt.Fprintf(cc.stdout, "%s/%s: %s\nMerge into %s:%s from %s:%s\n",
			baseOwner, baseRepo, title, baseOwner, baseBranch, headOwner, branch)
		if err != nil {
			return err
		}
		if body != "" {
			_, err = fmt.Fprintf(cc.stdout, "\n%s\n", body)
			if err != nil {
				return err
			}
		}
		return nil
	}
	// TODO(soon): Open editor to edit message.
	prURL, err := createPullRequest(ctx, cc.httpClient, pullRequestParams{
		authToken:  string(token),
		baseOwner:  baseOwner,
		baseRepo:   baseRepo,
		baseBranch: baseBranch,
		headOwner:  headOwner,
		headBranch: branch,
		title:      title,
		body:       body,
		disableMaintainerEdits: !*maintainerEdits,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cc.stdout, "Created pull request at %s\n", prURL)
	if err != nil {
		return err
	}
	return nil
}

func inferPullRequestMessage(ctx context.Context, git *gittool.Tool, base, head string) (title, body string, _ error) {
	// Read commit messages of divergent commits.
	const logFormat = "%B"
	p, err := git.Start(ctx, "log",
		"-z",
		"--reverse",
		"--no-merges",
		"--first-parent",
		"--pretty=tformat:%B",
		base+".."+head, "--")
	if err != nil {
		return "", "", fmt.Errorf("infer PR message: %v", err)
	}
	logData, err := ioutil.ReadAll(p)
	waitErr := p.Wait()
	if waitErr != nil {
		return "", "", fmt.Errorf("infer PR message: %v", waitErr)
	}
	if err != nil {
		return "", "", fmt.Errorf("infer PR message: %v", err)
	}

	// Split apart by commit (NUL byte).
	messages := bytes.SplitAfter(logData, []byte{0})
	if len(messages[len(messages)-1]) > 0 {
		return "", "", errors.New("infer PR message: parse log: unexpected EOF")
	}
	messages = messages[:len(messages)-1]
	if len(messages) == 0 {
		return "", "", errors.New("infer PR message: no divergent commits")
	}
	// Strip trailing NULs.
	for i := range messages {
		messages[i] = bytes.TrimSuffix(messages[i], []byte{0})
	}
	// First line of first commit message is the title.
	if i := bytes.IndexByte(messages[0], '\n'); i != -1 {
		title = string(bytes.TrimSpace(messages[0][:i]))
		messages[0] = messages[0][i+1:]
	} else {
		title = string(bytes.TrimSpace(messages[0]))
		messages[0] = nil
	}
	// Join rest of messages by bullets into body.
	bodyBuilder := new(strings.Builder)
	for i, msg := range messages {
		if i > 0 {
			bodyBuilder.WriteString("\n\n* ")
		}
		bodyBuilder.Write(bytes.TrimSpace(msg))
	}
	body = strings.TrimSpace(bodyBuilder.String())
	return title, body, nil
}

type pullRequestParams struct {
	authToken string

	baseOwner  string
	baseRepo   string
	baseBranch string

	headOwner  string
	headBranch string

	title string
	body  string

	disableMaintainerEdits bool
}

func createPullRequest(ctx context.Context, client *http.Client, params pullRequestParams) (prURL string, _ error) {
	if params.authToken == "" {
		return "", errors.New("create pull request: missing authentication token")
	}
	if params.baseOwner == "" || params.baseRepo == "" {
		return "", errors.New("create pull request: missing base owner or repository name")
	}
	if params.baseBranch == "" {
		return "", errors.New("create pull request: missing base branch")
	}
	if params.headOwner == "" || params.headBranch == "" {
		return "", errors.New("create pull request: missing head branch or owner")
	}
	if params.title == "" {
		return "", errors.New("create pull request: missing title")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls",
		url.PathEscape(params.baseOwner), url.PathEscape(params.baseRepo))
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("create pull request: %v", err)
	}
	req.Header.Set("User-Agent", userAgentString())
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Authorization", "token "+params.authToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	reqBody := map[string]interface{}{
		"title":                 params.title,
		"base":                  params.baseBranch,
		"head":                  params.headOwner + ":" + params.headBranch,
		"maintainer_can_modify": !params.disableMaintainerEdits,
	}
	if params.body != "" {
		reqBody["body"] = params.body
	}
	reqBodyJSON, err := json.Marshal(reqBody)
	req.Body = ioutil.NopCloser(bytes.NewReader(reqBodyJSON))

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("create pull request for %s/%s: %v", params.baseOwner, params.baseRepo, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		err := parseGitHubErrorResponse(resp)
		return "", fmt.Errorf("create pull request for %s/%s: %v", params.baseOwner, params.baseRepo, err)
	}
	var respDoc struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respDoc); err != nil {
		return "", fmt.Errorf("create pull request for %s/%s: parsing response: %v", params.baseOwner, params.baseRepo, err)
	}
	return respDoc.HTMLURL, nil
}

func parseGitHubErrorResponse(resp *http.Response) error {
	t, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || t != "application/json" {
		return fmt.Errorf("GitHub API HTTP %s", resp.Status)
	}
	var payload struct {
		Message string
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil || payload.Message == "" {
		return fmt.Errorf("GitHub API HTTP %s", resp.Status)
	}
	return fmt.Errorf("GitHub API HTTP %s: %s", resp.Status, payload.Message)
}

func parseGitHubRemoteURL(u string) (owner, repo string) {
	var path string
	switch {
	case strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "ssh://"):
		uu, err := url.Parse(u)
		if err != nil {
			return "", ""
		}
		if uu.Hostname() != "github.com" || uu.RawQuery != "" || uu.Fragment != "" {
			return "", ""
		}
		path = strings.TrimPrefix(uu.Path, "/")
	case strings.HasPrefix(u, "github.com:"):
		path = u[len("github.com:"):]
	case strings.HasPrefix(u, "git@github.com:"):
		path = u[len("git@github.com:"):]
	default:
		return "", ""
	}
	path = strings.TrimSuffix(path, ".git")
	i := strings.IndexByte(path, '/')
	if i == 0 || len(path)-i-1 == 0 {
		// One or part is empty.
		return "", ""
	}
	if i == -1 {
		return "", ""
	}
	if strings.Count(path[i+1:], "/") > 0 {
		return "", ""
	}
	return path[:i], path[i+1:]
}