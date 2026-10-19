package runtime

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type GitLogEntry struct {
	Commit      string `json:"commit"`
	ShortCommit string `json:"short_commit"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
	AuthoredAt  string `json:"authored_at"`
	Subject     string `json:"subject"`
}

type GitRemoteInfo struct {
	Name     string   `json:"name"`
	FetchURLs []string `json:"fetch_urls,omitempty"`
	PushURLs  []string `json:"push_urls,omitempty"`
}

type GitTagInfo struct {
	Name   string `json:"name"`
	Commit string `json:"commit"`
	Subject string `json:"subject,omitempty"`
}

type GitMutationResult struct {
	State     string   `json:"state"`
	Head      string   `json:"head,omitempty"`
	Applied   []string `json:"applied,omitempty"`
	Conflicts []string `json:"conflicts,omitempty"`
	Output    string   `json:"output,omitempty"`
}

type GitPushResult struct {
	Remote string   `json:"remote"`
	Branch string   `json:"branch,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Output string   `json:"output,omitempty"`
}

type GitFetchResult struct {
	Remote string `json:"remote"`
	Output string `json:"output,omitempty"`
}

func (r *Runtime) GitLog(ctx context.Context, ref string, maxCount int) ([]GitLogEntry, error) {
	if maxCount <= 0 {
		maxCount = 30
	}
	if maxCount > 200 {
		maxCount = 200
	}
	git, err := r.gitExecutable()
	if err != nil {
		return nil, err
	}
	args := []string{"log", "--no-decorate", "--max-count=" + strconv.Itoa(maxCount), "--format=%H%x00%h%x00%an%x00%ae%x00%aI%x00%s%x00"}
	if strings.TrimSpace(ref) != "" {
		commit, err := r.resolveGitCommit(ctx, git, ref)
		if err != nil {
			return nil, err
		}
		args = append(args, commit)
	}
	args = append(args, "--")
	cmd := r.readOnlyGitCommand(ctx, git, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	fields := bytes.Split(out, []byte{0})
	items := make([]GitLogEntry, 0, len(fields)/6)
	for i := 0; i+5 < len(fields); i += 6 {
		if len(fields[i]) == 0 {
			continue
		}
		items = append(items, GitLogEntry{
			Commit:      strings.TrimSpace(string(fields[i])),
			ShortCommit: strings.TrimSpace(string(fields[i+1])),
			AuthorName:  strings.TrimSpace(string(fields[i+2])),
			AuthorEmail: strings.TrimSpace(string(fields[i+3])),
			AuthoredAt:  strings.TrimSpace(string(fields[i+4])),
			Subject:     strings.TrimSpace(string(fields[i+5])),
		})
	}
	return items, nil
}

func (r *Runtime) GitShow(ctx context.Context, ref string, maxOutputBytes int) (string, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return "", err
	}
	commit, err := r.resolveGitCommit(ctx, git, ref)
	if err != nil {
		return "", err
	}
	return r.gitOutputLimited(ctx, git, maxOutputBytes,
		"show", "--no-ext-diff", "--no-textconv", "--ignore-submodules=all",
		"--format=fuller", "--stat", "--patch", commit, "--")
}

func (r *Runtime) GitRemoteList(ctx context.Context) ([]GitRemoteInfo, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return nil, err
	}
	namesOutput, err := r.readOnlyGitCommand(ctx, git, "remote").Output()
	if err != nil {
		return nil, fmt.Errorf("git remote: %w", err)
	}
	var result []GitRemoteInfo
	for _, raw := range strings.Split(strings.TrimSpace(string(namesOutput)), "\n") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if err := validateGitRemoteName(name); err != nil {
			return nil, err
		}
		fetchURLs, err := r.gitRemoteURLs(ctx, git, name, false)
		if err != nil {
			return nil, err
		}
		pushURLs, err := r.gitRemoteURLs(ctx, git, name, true)
		if err != nil {
			return nil, err
		}
		result = append(result, GitRemoteInfo{
			Name:      name,
			FetchURLs: sanitizeGitRemoteDisplayURLs(fetchURLs),
			PushURLs:  sanitizeGitRemoteDisplayURLs(pushURLs),
		})
	}
	return result, nil
}

func (r *Runtime) GitTagList(ctx context.Context, maxCount int) ([]GitTagInfo, error) {
	if maxCount <= 0 {
		maxCount = 100
	}
	if maxCount > 500 {
		maxCount = 500
	}
	git, err := r.gitExecutable()
	if err != nil {
		return nil, err
	}
	cmd := r.readOnlyGitCommand(ctx, git,
		"for-each-ref",
		"--count="+strconv.Itoa(maxCount),
		"--sort=-creatordate",
		"--format=%(refname:strip=2)%00%(*objectname)%00%(objectname)%00%(subject)%00",
		"refs/tags")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git tag list: %w", err)
	}
	fields := bytes.Split(out, []byte{0})
	items := make([]GitTagInfo, 0, len(fields)/4)
	for i := 0; i+3 < len(fields); i += 4 {
		name := strings.TrimSpace(string(fields[i]))
		if name == "" {
			continue
		}
		peeled := strings.TrimSpace(string(fields[i+1]))
		object := strings.TrimSpace(string(fields[i+2]))
		if peeled != "" {
			object = peeled
		}
		items = append(items, GitTagInfo{Name: name, Commit: object, Subject: strings.TrimSpace(string(fields[i+3]))})
	}
	return items, nil
}

func (r *Runtime) GitTagCreate(ctx context.Context, name, ref, message string, annotated bool) (GitTagInfo, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return GitTagInfo{}, err
	}
	if err := r.requireGitTopLevel(ctx, git); err != nil {
		return GitTagInfo{}, err
	}
	name, err = r.validateGitTagName(ctx, git, name)
	if err != nil {
		return GitTagInfo{}, err
	}
	commit, err := r.resolveGitCommit(ctx, git, ref)
	if err != nil {
		return GitTagInfo{}, err
	}
	args := []string{"tag"}
	if annotated {
		message = strings.TrimSpace(message)
		if message == "" {
			return GitTagInfo{}, fmt.Errorf("annotated tag message is required")
		}
		if len(message) > 4096 {
			return GitTagInfo{}, fmt.Errorf("tag message exceeds 4096 bytes")
		}
		args = append(args, "-a", "--no-sign", "-m", message)
	}
	args = append(args, name, commit)
	cmd := r.controlledGitCommand(ctx, git, args...)
	if annotated {
		authorName, authorEmail, _, _, identityErr := r.gitCommitIdentity(ctx, git)
		if identityErr != nil {
			return GitTagInfo{}, identityErr
		}
		cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_COMMITTER_NAME", authorName)
		cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_COMMITTER_EMAIL", authorEmail)
	}
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		return GitTagInfo{}, fmt.Errorf("git tag create: %s", strings.TrimSpace(string(output)))
	}
	return GitTagInfo{Name: name, Commit: commit, Subject: strings.TrimSpace(message)}, nil
}

func (r *Runtime) GitTagDelete(ctx context.Context, name string) (GitTagInfo, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return GitTagInfo{}, err
	}
	if err := r.requireGitTopLevel(ctx, git); err != nil {
		return GitTagInfo{}, err
	}
	name, err = r.validateGitTagName(ctx, git, name)
	if err != nil {
		return GitTagInfo{}, err
	}
	commit, err := r.resolveGitCommit(ctx, git, "refs/tags/"+name)
	if err != nil {
		return GitTagInfo{}, err
	}
	cmd := r.controlledGitCommand(ctx, git, "tag", "-d", "--", name)
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		return GitTagInfo{}, fmt.Errorf("git tag delete: %s", strings.TrimSpace(string(output)))
	}
	return GitTagInfo{Name: name, Commit: commit}, nil
}

func (r *Runtime) GitCherryPick(ctx context.Context, refs []string) (GitMutationResult, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return GitMutationResult{}, err
	}
	if err := r.requireGitTopLevel(ctx, git); err != nil {
		return GitMutationResult{}, err
	}
	if len(refs) == 0 {
		return GitMutationResult{}, fmt.Errorf("at least one commit is required")
	}
	if len(refs) > 20 {
		return GitMutationResult{}, fmt.Errorf("git_cherry_pick accepts at most 20 commits")
	}
	if err := r.requireGitClean(ctx, git); err != nil {
		return GitMutationResult{}, err
	}
	resolved := make([]string, 0, len(refs))
	for _, ref := range refs {
		commit, resolveErr := r.resolveGitCommit(ctx, git, ref)
		if resolveErr != nil {
			return GitMutationResult{}, resolveErr
		}
		if safetyErr := r.ensureGitMutationCommitSafe(ctx, git, commit); safetyErr != nil {
			return GitMutationResult{}, safetyErr
		}
		resolved = append(resolved, commit)
	}
	cmd, err := r.committerGitCommand(ctx, git, append([]string{"cherry-pick"}, resolved...)...)
	if err != nil {
		return GitMutationResult{}, err
	}
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		if r.gitPseudoRefExists(ctx, git, "CHERRY_PICK_HEAD") {
			return r.gitMutationState(ctx, git, "conflict", resolved, string(output)), nil
		}
		return GitMutationResult{}, fmt.Errorf("git cherry-pick: %s", strings.TrimSpace(string(output)))
	}
	return r.gitMutationState(ctx, git, "completed", resolved, string(output)), nil
}

func (r *Runtime) GitCherryPickContinue(ctx context.Context) (GitMutationResult, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return GitMutationResult{}, err
	}
	if !r.gitPseudoRefExists(ctx, git, "CHERRY_PICK_HEAD") {
		return GitMutationResult{}, fmt.Errorf("no cherry-pick is in progress")
	}
	if conflicts, err := r.gitConflictPaths(ctx, git); err != nil {
		return GitMutationResult{}, err
	} else if len(conflicts) != 0 {
		return GitMutationResult{State: "conflict", Conflicts: conflicts}, fmt.Errorf("cherry-pick still has unresolved conflicts")
	}
	if err := r.ensureGitContinuationSafe(ctx, git); err != nil {
		return GitMutationResult{}, err
	}
	cmd, err := r.committerGitCommand(ctx, git, "cherry-pick", "--continue")
	if err != nil {
		return GitMutationResult{}, err
	}
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return GitMutationResult{}, fmt.Errorf("git cherry-pick --continue: %s", strings.TrimSpace(string(output)))
	}
	return r.gitMutationState(ctx, git, "completed", nil, string(output)), nil
}

func (r *Runtime) GitCherryPickAbort(ctx context.Context) (GitMutationResult, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return GitMutationResult{}, err
	}
	if !r.gitPseudoRefExists(ctx, git, "CHERRY_PICK_HEAD") {
		return GitMutationResult{}, fmt.Errorf("no cherry-pick is in progress")
	}
	cmd := r.controlledGitCommand(ctx, git, "cherry-pick", "--abort")
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return GitMutationResult{}, fmt.Errorf("git cherry-pick --abort: %s", strings.TrimSpace(string(output)))
	}
	return r.gitMutationState(ctx, git, "aborted", nil, string(output)), nil
}

func (r *Runtime) GitMerge(ctx context.Context, ref string, noFF bool) (GitMutationResult, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return GitMutationResult{}, err
	}
	if err := r.requireGitTopLevel(ctx, git); err != nil {
		return GitMutationResult{}, err
	}
	if err := r.requireGitClean(ctx, git); err != nil {
		return GitMutationResult{}, err
	}
	target, err := r.resolveGitCommit(ctx, git, ref)
	if err != nil {
		return GitMutationResult{}, err
	}
	if err := r.ensureGitMergeTargetSafe(ctx, git, target); err != nil {
		return GitMutationResult{}, err
	}
	args := []string{"merge", "--no-edit", "--no-gpg-sign"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, target)
	cmd, err := r.identityGitCommand(ctx, git, args...)
	if err != nil {
		return GitMutationResult{}, err
	}
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		if r.gitPseudoRefExists(ctx, git, "MERGE_HEAD") {
			return r.gitMutationState(ctx, git, "conflict", []string{target}, string(output)), nil
		}
		return GitMutationResult{}, fmt.Errorf("git merge: %s", strings.TrimSpace(string(output)))
	}
	return r.gitMutationState(ctx, git, "completed", []string{target}, string(output)), nil
}

func (r *Runtime) GitMergeAbort(ctx context.Context) (GitMutationResult, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return GitMutationResult{}, err
	}
	if !r.gitPseudoRefExists(ctx, git, "MERGE_HEAD") {
		return GitMutationResult{}, fmt.Errorf("no merge is in progress")
	}
	cmd := r.controlledGitCommand(ctx, git, "merge", "--abort")
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return GitMutationResult{}, fmt.Errorf("git merge --abort: %s", strings.TrimSpace(string(output)))
	}
	return r.gitMutationState(ctx, git, "aborted", nil, string(output)), nil
}

func (r *Runtime) GitFetch(ctx context.Context, remote string, tags, prune bool, hostCredentials bool) (GitFetchResult, error) {
	return r.gitFetch(ctx, remote, tags, prune, hostCredentials, !hostCredentials)
}

// GitFetchAgent keeps the Agent on an explicitly configured HTTPS remote while
// allowing the host credential manager to satisfy authentication. Credentials
// are never returned to the caller, arbitrary URLs/refspecs are not accepted,
// and SSH/local/file transports remain blocked.
func (r *Runtime) GitFetchAgent(ctx context.Context, remote string, tags, prune bool) (GitFetchResult, error) {
	return r.gitFetch(ctx, remote, tags, prune, true, true)
}

func (r *Runtime) gitFetch(ctx context.Context, remote string, tags, prune, hostCredentials, requireSafeRemote bool) (GitFetchResult, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return GitFetchResult{}, err
	}
	if err := r.requireGitTopLevel(ctx, git); err != nil {
		return GitFetchResult{}, err
	}
	remote = strings.TrimSpace(remote)
	if remote == "" {
		remote = "origin"
	}
	if err := validateGitRemoteName(remote); err != nil {
		return GitFetchResult{}, err
	}
	if requireSafeRemote {
		if err := r.requireAgentSafeGitRemote(ctx, git, remote, false); err != nil {
			return GitFetchResult{}, err
		}
	}
	args := []string{"fetch", "--no-recurse-submodules"}
	if prune {
		args = append(args, "--prune")
	}
	if tags {
		args = append(args, "--tags")
	} else {
		args = append(args, "--no-tags")
	}
	args = append(args, remote)
	cmd := r.remoteGitCommand(ctx, git, hostCredentials, args...)
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return GitFetchResult{}, fmt.Errorf("git fetch: %s", strings.TrimSpace(string(output)))
	}
	return GitFetchResult{Remote: remote, Output: strings.TrimSpace(string(output))}, nil
}

func (r *Runtime) GitPush(ctx context.Context, remote, branch string, tags []string, setUpstream, hostCredentials bool) (GitPushResult, error) {
	return r.gitPush(ctx, remote, branch, tags, setUpstream, hostCredentials, !hostCredentials)
}

// GitPushAgent uses only an existing configured HTTPS remote and fixed branch/
// tag refspecs. It may use the host credential manager for authentication, but
// does not expose force push, deletion, SSH, local transports, or credentials.
func (r *Runtime) GitPushAgent(ctx context.Context, remote, branch string, tags []string, setUpstream bool) (GitPushResult, error) {
	return r.gitPush(ctx, remote, branch, tags, setUpstream, true, true)
}

func (r *Runtime) gitPush(ctx context.Context, remote, branch string, tags []string, setUpstream, hostCredentials, requireSafeRemote bool) (GitPushResult, error) {
	git, err := r.gitExecutable()
	if err != nil {
		return GitPushResult{}, err
	}
	if err := r.requireGitTopLevel(ctx, git); err != nil {
		return GitPushResult{}, err
	}
	remote = strings.TrimSpace(remote)
	if remote == "" {
		remote = "origin"
	}
	if err := validateGitRemoteName(remote); err != nil {
		return GitPushResult{}, err
	}
	if requireSafeRemote {
		if err := r.requireAgentSafeGitRemote(ctx, git, remote, true); err != nil {
			return GitPushResult{}, err
		}
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		current, branchErr := r.GitBranch(ctx)
		if branchErr != nil {
			return GitPushResult{}, branchErr
		}
		branch = current
	}
	if err := r.validateGitBranchName(ctx, git, branch); err != nil {
		return GitPushResult{}, err
	}
	refspecs := []string{"refs/heads/" + branch + ":refs/heads/" + branch}
	cleanTags := make([]string, 0, len(tags))
	if len(tags) > 50 {
		return GitPushResult{}, fmt.Errorf("git_push accepts at most 50 tags")
	}
	for _, name := range tags {
		tagName, tagErr := r.validateGitTagName(ctx, git, name)
		if tagErr != nil {
			return GitPushResult{}, tagErr
		}
		if _, resolveErr := r.resolveGitCommit(ctx, git, "refs/tags/"+tagName); resolveErr != nil {
			return GitPushResult{}, resolveErr
		}
		refspecs = append(refspecs, "refs/tags/"+tagName+":refs/tags/"+tagName)
		cleanTags = append(cleanTags, tagName)
	}
	args := []string{"push", "--porcelain"}
	if setUpstream {
		args = append(args, "--set-upstream")
	}
	args = append(args, remote)
	args = append(args, refspecs...)
	cmd := r.remoteGitCommand(ctx, git, hostCredentials, args...)
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return GitPushResult{}, fmt.Errorf("git push: %s", strings.TrimSpace(string(output)))
	}
	return GitPushResult{
		Remote: remote,
		Branch: branch,
		Tags:   cleanTags,
		Output: strings.TrimSpace(string(output)),
	}, nil
}

func (r *Runtime) gitExecutable() (string, error) {
	if !r.gitSupported(context.Background()) {
		return "", fmt.Errorf("Git is unsupported for this Environment root")
	}
	return exec.LookPath("git")
}

func (r *Runtime) requireGitTopLevel(ctx context.Context, git string) error {
	topCommand := r.readOnlyGitCommand(ctx, git, "rev-parse", "--show-toplevel")
	topOutput, err := topCommand.Output()
	if err != nil {
		return fmt.Errorf("Git operation is unsupported for this Environment root")
	}
	top, err := canonicalDir(strings.TrimSpace(string(topOutput)))
	if err != nil {
		return err
	}
	if !samePath(top, r.root) {
		return fmt.Errorf("Git mutation requires the Environment root to equal the Git top-level")
	}
	return nil
}

func (r *Runtime) resolveGitCommit(ctx context.Context, git, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = "HEAD"
	}
	if len(ref) > 512 || strings.ContainsAny(ref, "\x00\r\n") {
		return "", fmt.Errorf("invalid Git ref")
	}
	cmd := r.readOnlyGitCommand(ctx, git, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Git commit %q not found", ref)
	}
	value := strings.TrimSpace(string(out))
	if len(value) != 40 && len(value) != 64 {
		return "", fmt.Errorf("Git resolved an unexpected commit id")
	}
	return value, nil
}

func (r *Runtime) validateGitTagName(ctx context.Context, git, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 250 || strings.ContainsAny(name, "\x00\r\n") {
		return "", fmt.Errorf("invalid Git tag name")
	}
	cmd := r.readOnlyGitCommand(ctx, git, "check-ref-format", "refs/tags/"+name)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("invalid Git tag name %q", name)
	}
	return name, nil
}

func (r *Runtime) validateGitBranchName(ctx context.Context, git, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 250 || strings.ContainsAny(name, "\x00\r\n") {
		return fmt.Errorf("invalid Git branch name")
	}
	cmd := r.readOnlyGitCommand(ctx, git, "check-ref-format", "--branch", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("invalid Git branch name %q", name)
	}
	return nil
}

func validateGitRemoteName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 || strings.ContainsAny(name, "\x00\r\n") || strings.HasPrefix(name, "-") {
		return fmt.Errorf("invalid Git remote name")
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._-/", r) {
			continue
		}
		return fmt.Errorf("invalid Git remote name %q", name)
	}
	return nil
}

func (r *Runtime) requireGitClean(ctx context.Context, git string) error {
	cmd := r.readOnlyGitCommand(ctx, git, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=all", "--", ".")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if len(bytes.Trim(out, "\x00\r\n\t ")) != 0 {
		return fmt.Errorf("Git mutation requires a clean working tree")
	}
	return nil
}

func (r *Runtime) ensureGitMutationCommitSafe(ctx context.Context, git, commit string) error {
	if err := r.ensureGitFiltersInactive(ctx, git); err != nil {
		return err
	}
	cmd := r.readOnlyGitCommand(ctx, git, "diff-tree", "--root", "--no-commit-id", "--name-only", "-r", "-z", commit)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("inspect cherry-pick paths: %w", err)
	}
	paths := splitNULPaths(out)
	return r.ensureGitIncomingPathsSafe(ctx, git, paths)
}

func (r *Runtime) ensureGitContinuationSafe(ctx context.Context, git string) error {
	if err := r.ensureGitFiltersInactive(ctx, git); err != nil {
		return err
	}
	staged := r.readOnlyGitCommand(ctx, git,
		"diff", "--cached", "--name-only", "-z",
		"--no-ext-diff", "--no-textconv", "--ignore-submodules=all", "--")
	out, err := staged.Output()
	if err != nil {
		return fmt.Errorf("inspect staged cherry-pick paths: %w", err)
	}
	return r.ensureGitIncomingPathsSafe(ctx, git, splitNULPaths(out))
}

func (r *Runtime) ensureGitMergeTargetSafe(ctx context.Context, git, target string) error {
	if err := r.ensureGitFiltersInactive(ctx, git); err != nil {
		return err
	}
	baseCmd := r.readOnlyGitCommand(ctx, git, "merge-base", "HEAD", target)
	baseOut, err := baseCmd.Output()
	if err != nil {
		return fmt.Errorf("git merge-base: %w", err)
	}
	base := strings.TrimSpace(string(baseOut))
	pathsCmd := r.readOnlyGitCommand(ctx, git, "diff", "--name-only", "-z", "--no-ext-diff", "--no-textconv", base, target, "--")
	out, err := pathsCmd.Output()
	if err != nil {
		return fmt.Errorf("inspect merge paths: %w", err)
	}
	return r.ensureGitIncomingPathsSafe(ctx, git, splitNULPaths(out))
}

func (r *Runtime) ensureGitIncomingPathsSafe(ctx context.Context, git string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	for _, path := range paths {
		if filepath.Base(filepath.Clean(path)) == ".gitattributes" {
			return fmt.Errorf("Git mutation refuses commits that change .gitattributes because filters or merge drivers could execute host code")
		}
		if _, err := r.lexical(path); err != nil {
			return fmt.Errorf("unsafe Git path %q: %w", path, err)
		}
	}
	payload := bytes.Join(stringsToBytes(paths), []byte{0})
	payload = append(payload, 0)
	check := r.readOnlyGitCommand(ctx, git, "check-attr", "-z", "--stdin", "filter", "merge")
	check.Stdin = bytes.NewReader(payload)
	out, err := check.Output()
	if err != nil {
		return fmt.Errorf("git check-attr mutation safety: %w", err)
	}
	fields := bytes.Split(out, []byte{0})
	for i := 0; i+2 < len(fields); i += 3 {
		path := strings.TrimSpace(string(fields[i]))
		attr := strings.TrimSpace(string(fields[i+1]))
		value := strings.TrimSpace(string(fields[i+2]))
		switch attr {
		case "filter":
			if value != "" && value != "unspecified" && value != "unset" {
				return fmt.Errorf("Git mutation refuses path %q because filter attribute %q may execute host code", path, value)
			}
		case "merge":
			if value != "" && value != "unspecified" && value != "unset" && value != "set" {
				return fmt.Errorf("Git mutation refuses path %q because custom merge driver %q may execute host code", path, value)
			}
		}
	}
	return nil
}

func stringsToBytes(items []string) [][]byte {
	out := make([][]byte, 0, len(items))
	for _, item := range items {
		out = append(out, []byte(item))
	}
	return out
}

func splitNULPaths(out []byte) []string {
	fields := bytes.Split(out, []byte{0})
	items := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) != 0 {
			items = append(items, string(field))
		}
	}
	return items
}

func (r *Runtime) identityGitCommand(ctx context.Context, git string, args ...string) (*exec.Cmd, error) {
	authorName, authorEmail, committerName, committerEmail, err := r.gitCommitIdentity(ctx, git)
	if err != nil {
		return nil, err
	}
	cmd := r.controlledGitCommand(ctx, git, args...)
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_AUTHOR_NAME", authorName)
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_AUTHOR_EMAIL", authorEmail)
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_COMMITTER_NAME", committerName)
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_COMMITTER_EMAIL", committerEmail)
	return cmd, nil
}

func (r *Runtime) committerGitCommand(ctx context.Context, git string, args ...string) (*exec.Cmd, error) {
	_, _, committerName, committerEmail, err := r.gitCommitIdentity(ctx, git)
	if err != nil {
		return nil, err
	}
	cmd := r.controlledGitCommand(ctx, git, args...)
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_COMMITTER_NAME", committerName)
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_COMMITTER_EMAIL", committerEmail)
	return cmd, nil
}

func (r *Runtime) gitPseudoRefExists(ctx context.Context, git, ref string) bool {
	cmd := r.readOnlyGitCommand(ctx, git, "rev-parse", "-q", "--verify", ref)
	return cmd.Run() == nil
}

func (r *Runtime) gitConflictPaths(ctx context.Context, git string) ([]string, error) {
	cmd := r.readOnlyGitCommand(ctx, git, "diff", "--name-only", "-z", "--diff-filter=U", "--")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("inspect Git conflicts: %w", err)
	}
	return splitNULPaths(out), nil
}

func (r *Runtime) gitMutationState(ctx context.Context, git, state string, applied []string, output string) GitMutationResult {
	headCmd := r.readOnlyGitCommand(ctx, git, "rev-parse", "HEAD")
	headOut, _ := headCmd.Output()
	conflicts, _ := r.gitConflictPaths(ctx, git)
	return GitMutationResult{
		State:     state,
		Head:      strings.TrimSpace(string(headOut)),
		Applied:   append([]string(nil), applied...),
		Conflicts: conflicts,
		Output:    strings.TrimSpace(output),
	}
}

func (r *Runtime) gitOutputLimited(ctx context.Context, git string, maxOutputBytes int, args ...string) (string, error) {
	if maxOutputBytes <= 0 {
		maxOutputBytes = 256 * 1024
	}
	if maxOutputBytes > 4*1024*1024 {
		maxOutputBytes = 4 * 1024 * 1024
	}
	cmd := r.readOnlyGitCommand(ctx, git, args...)
	buffer := &limitedBuffer{limit: maxOutputBytes}
	cmd.Stdout = buffer
	cmd.Stderr = buffer
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(buffer.String()))
	}
	return buffer.String(), nil
}

func (r *Runtime) gitRemoteURLs(ctx context.Context, git, remote string, push bool) ([]string, error) {
	args := []string{"remote", "get-url"}
	if push {
		args = append(args, "--push")
	}
	args = append(args, "--all", remote)
	cmd := r.readOnlyGitCommand(ctx, git, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git remote get-url %q: %w", remote, err)
	}
	var urls []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		value := strings.TrimSpace(line)
		if value != "" {
			urls = append(urls, value)
		}
	}
	return urls, nil
}

func sanitizeGitRemoteDisplayURLs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, sanitizeGitRemoteDisplayURL(value))
	}
	return out
}

func sanitizeGitRemoteDisplayURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) || strings.HasPrefix(raw, ".") || strings.HasPrefix(strings.ToLower(raw), "file://") {
		return "<local-path>"
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "<redacted-remote>"
		}
		parsed.User = nil
		parsed.RawQuery = ""
		parsed.ForceQuery = false
		parsed.Fragment = ""
		return parsed.String()
	}
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		raw = raw[at+1:]
	}
	if strings.Contains(raw, ":") && !strings.ContainsAny(raw, "\r\n\x00") {
		return raw
	}
	return "<redacted-remote>"
}

func (r *Runtime) requireAgentSafeGitRemote(ctx context.Context, git, remote string, push bool) error {
	urls, err := r.gitRemoteURLs(ctx, git, remote, push)
	if err != nil {
		return err
	}
	if len(urls) == 0 {
		return fmt.Errorf("Git remote %q has no URL", remote)
	}
	for _, raw := range urls {
		parsed, parseErr := url.Parse(raw)
		if parseErr != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
			return fmt.Errorf("Agent-safe remote Git requires HTTPS URLs; %q is not allowed", raw)
		}
		if parsed.User != nil {
			return fmt.Errorf("Agent-safe remote Git refuses credentials embedded in remote URLs")
		}
	}
	return nil
}

func (r *Runtime) remoteGitCommand(ctx context.Context, git string, hostCredentials bool, args ...string) *exec.Cmd {
	fullArgs := []string{
		"--no-pager",
		"-c", "core.fsmonitor=false",
		"-c", "core.attributesFile=" + os.DevNull,
		"-c", "core.excludesFile=" + os.DevNull,
		"-c", "core.hooksPath=" + os.DevNull,
		"-c", "commit.gpgSign=false",
		"-c", "tag.gpgSign=false",
		"-c", "gc.auto=0",
		"-c", "maintenance.auto=false",
		"-c", "submodule.recurse=false",
		"-C", r.root,
		"--work-tree=" + r.root,
	}
	if !hostCredentials {
		fullArgs = append(fullArgs,
			"-c", "protocol.allow=never",
			"-c", "protocol.https.allow=always",
			"-c", "credential.helper=",
			"-c", "core.askPass=",
		)
	}
	fullArgs = append(fullArgs, args...)
	cmd := exec.CommandContext(ctx, git, fullArgs...)
	configureCommand(cmd)
	cmd.Env = sanitizeGitEnvironment(r.baseEnvironment)
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_CONFIG_NOSYSTEM", "1")
	if !hostCredentials {
		cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_CONFIG_GLOBAL", os.DevNull)
		cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_ASKPASS", "")
		cmd.Env = replaceEnvironmentValue(cmd.Env, "SSH_ASKPASS", "")
	}
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_ATTR_NOSYSTEM", "1")
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_TERMINAL_PROMPT", "0")
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_OPTIONAL_LOCKS", "1")
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_EXTERNAL_DIFF", "")
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_PAGER", "cat")
	cmd.Env = replaceEnvironmentValue(cmd.Env, "GIT_EDITOR", "true")
	return cmd
}
