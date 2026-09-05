package runtime

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const hardGitChangeSetBytes = 20 << 20

type GitUntrackedFile struct {
	Path string `json:"path"`
	Data []byte `json:"data"`
	Mode uint32 `json:"mode,omitempty"`
}

type GitChangeSet struct {
	StagedPatch   []byte             `json:"staged_patch,omitempty"`
	UnstagedPatch []byte             `json:"unstaged_patch,omitempty"`
	Untracked     []GitUntrackedFile `json:"untracked,omitempty"`
}

type GitPushStatus struct {
	Head        string `json:"head"`
	Branch      string `json:"branch,omitempty"`
	HasUpstream bool   `json:"has_upstream"`
	Upstream    string `json:"upstream,omitempty"`
	Ahead       int    `json:"ahead"`
}

func (r *Native) GitExportChanges() (GitChangeSet, error) {
	staged, err := r.gitPatch("diff", "--cached", "--binary", "--full-index", "--no-ext-diff", "--")
	if err != nil {
		return GitChangeSet{}, err
	}
	unstaged, err := r.gitPatch("diff", "--binary", "--full-index", "--no-ext-diff", "--")
	if err != nil {
		return GitChangeSet{}, err
	}
	set := GitChangeSet{StagedPatch: staged, UnstagedPatch: unstaged}
	total := len(staged) + len(unstaged)
	if total > hardGitChangeSetBytes {
		return GitChangeSet{}, &RuntimeError{Kind: ErrLimitExceeded}
	}
	status, err := r.GitStatus()
	if err != nil {
		return GitChangeSet{}, err
	}
	for _, entry := range status {
		if entry.X != "?" || entry.Y != "?" {
			continue
		}
		lexicalTarget, err := r.guard.lexical(entry.Path)
		if err != nil {
			return GitChangeSet{}, err
		}
		info, err := os.Lstat(lexicalTarget)
		if err != nil {
			return GitChangeSet{}, &RuntimeError{Kind: ErrIO, Path: entry.Path, Err: err}
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return GitChangeSet{}, &RuntimeError{Kind: ErrInvalidPath, Path: entry.Path}
		}
		data, _, err := r.Read(entry.Path, hardReadBytes)
		if err != nil {
			return GitChangeSet{}, err
		}
		total += len(data)
		if total > hardGitChangeSetBytes {
			return GitChangeSet{}, &RuntimeError{Kind: ErrLimitExceeded}
		}
		set.Untracked = append(set.Untracked, GitUntrackedFile{Path: entry.Path, Data: data, Mode: uint32(info.Mode().Perm())})
	}
	return set, nil
}

func (r *Native) GitApplyChanges(set GitChangeSet) error {
	total := len(set.StagedPatch) + len(set.UnstagedPatch)
	for _, file := range set.Untracked {
		total += len(file.Data)
	}
	if total > hardGitChangeSetBytes {
		return &RuntimeError{Kind: ErrLimitExceeded}
	}
	if len(set.StagedPatch) > 0 {
		result, err := r.gitExecInput(set.StagedPatch, "apply", "--index", "--whitespace=nowarn", "-")
		if err != nil {
			return err
		}
		if result.ExitCode != 0 {
			return gitResultError("apply --index", result)
		}
	}
	if len(set.UnstagedPatch) > 0 {
		result, err := r.gitExecInput(set.UnstagedPatch, "apply", "--whitespace=nowarn", "-")
		if err != nil {
			return err
		}
		if result.ExitCode != 0 {
			return gitResultError("apply", result)
		}
	}
	for _, file := range set.Untracked {
		target, _, err := r.guard.WriteTarget(file.Path)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(target); err == nil {
			return &RuntimeError{Kind: ErrInvalidPath, Path: file.Path, Err: fmt.Errorf("untracked target already exists")}
		} else if !os.IsNotExist(err) {
			return &RuntimeError{Kind: ErrIO, Path: file.Path, Err: err}
		}
		if _, err := r.Write(file.Path, file.Data, true); err != nil {
			return err
		}
		if file.Mode != 0 {
			resolved, _, err := r.guard.Existing(file.Path)
			if err != nil {
				return err
			}
			if err := os.Chmod(resolved, os.FileMode(file.Mode)&0o777); err != nil {
				return &RuntimeError{Kind: ErrIO, Path: file.Path, Err: err}
			}
		}
	}
	return nil
}

func (r *Native) GitPushStatus() (GitPushStatus, error) {
	head, err := r.GitResolveRef("HEAD")
	if err != nil {
		return GitPushStatus{}, err
	}
	branch, err := r.GitBranch()
	if err != nil {
		return GitPushStatus{}, err
	}
	status := GitPushStatus{Head: head, Branch: branch}
	if strings.TrimSpace(branch) == "" {
		return status, nil
	}
	upstreamResult, err := r.gitExec("for-each-ref", "--format=%(upstream:short)", "refs/heads/"+branch)
	if err != nil {
		return GitPushStatus{}, err
	}
	if upstreamResult.ExitCode != 0 {
		return GitPushStatus{}, gitResultError("for-each-ref upstream", upstreamResult)
	}
	upstream := strings.TrimSpace(upstreamResult.Stdout)
	if upstream == "" {
		return status, nil
	}
	status.HasUpstream = true
	status.Upstream = upstream
	aheadResult, err := r.gitExec("rev-list", "--count", upstream+"..HEAD")
	if err != nil {
		return GitPushStatus{}, err
	}
	if aheadResult.ExitCode != 0 {
		return GitPushStatus{}, gitResultError("rev-list upstream..HEAD", aheadResult)
	}
	ahead, err := strconv.Atoi(strings.TrimSpace(aheadResult.Stdout))
	if err != nil || ahead < 0 {
		return GitPushStatus{}, &RuntimeError{Kind: ErrIO, Path: upstream, Err: fmt.Errorf("invalid ahead count %q", strings.TrimSpace(aheadResult.Stdout))}
	}
	status.Ahead = ahead
	return status, nil
}

func (r *Native) gitPatch(args ...string) ([]byte, error) {
	result, err := r.Exec(Command{Executable: "git", Args: args, Cwd: r.root, MaxOutputBytes: hardOutputBytes})
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, gitResultError(strings.Join(args, " "), result)
	}
	if result.StdoutTruncated {
		return nil, &RuntimeError{Kind: ErrLimitExceeded}
	}
	return []byte(result.Stdout), nil
}

func (r *Native) gitExecInput(stdin []byte, args ...string) (CommandResult, error) {
	return r.Exec(Command{Executable: "git", Args: args, Cwd: r.root, Stdin: stdin})
}
