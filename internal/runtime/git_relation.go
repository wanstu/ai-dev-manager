package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

type GitRelation struct {
	LeftCommit  string `json:"left_commit"`
	RightCommit string `json:"right_commit"`
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
	Diverged    bool   `json:"diverged"`
}

func (r *Native) GitRelation(left, right string) (GitRelation, error) {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if !safeGitRefInput(left) {
		return GitRelation{}, &RuntimeError{Kind: ErrInvalidPath, Path: left}
	}
	if !safeGitRefInput(right) {
		return GitRelation{}, &RuntimeError{Kind: ErrInvalidPath, Path: right}
	}
	leftCommit, err := r.GitResolveRef(left)
	if err != nil {
		return GitRelation{}, err
	}
	rightCommit, err := r.GitResolveRef(right)
	if err != nil {
		return GitRelation{}, err
	}
	result, err := r.gitExec("rev-list", "--left-right", "--count", leftCommit+"..."+rightCommit)
	if err != nil {
		return GitRelation{}, err
	}
	if result.ExitCode != 0 {
		return GitRelation{}, gitResultError("rev-list --left-right --count", result)
	}
	fields := strings.Fields(strings.TrimSpace(result.Stdout))
	if len(fields) != 2 {
		return GitRelation{}, &RuntimeError{Kind: ErrIO, Path: left + "..." + right, Err: fmt.Errorf("invalid relation count %q", strings.TrimSpace(result.Stdout))}
	}
	ahead, err := strconv.Atoi(fields[0])
	if err != nil || ahead < 0 {
		return GitRelation{}, &RuntimeError{Kind: ErrIO, Path: left, Err: fmt.Errorf("invalid ahead count %q", fields[0])}
	}
	behind, err := strconv.Atoi(fields[1])
	if err != nil || behind < 0 {
		return GitRelation{}, &RuntimeError{Kind: ErrIO, Path: right, Err: fmt.Errorf("invalid behind count %q", fields[1])}
	}
	return GitRelation{
		LeftCommit:  leftCommit,
		RightCommit: rightCommit,
		Ahead:       ahead,
		Behind:      behind,
		Diverged:    ahead > 0 && behind > 0,
	}, nil
}
