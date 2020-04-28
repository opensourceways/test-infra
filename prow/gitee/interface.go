package gitee

import (
	sdk "gitee.com/openeuler/go-gitee/gitee"
	"k8s.io/test-infra/prow/github"
)

// Client interface for Gitee API
type Client interface {
	github.UserClient

	CreatePullRequest(org, repo, title, body, head, base string, canModify bool) (sdk.PullRequest, error)
	GetPullRequests(org, repo, state, head, base string) ([]sdk.PullRequest, error)
	UpdatePullRequest(org, repo string, number int32, title, body, state, labels string) (sdk.PullRequest, error)

	ListCollaborators(org, repo string) ([]github.User, error)
	GetRef(org, repo, ref string) (string, error)
	GetPullRequestChanges(org, repo string, number int) ([]github.PullRequestChange, error)
	GetPRLabels(org, repo string, number int) ([]sdk.Label, error)
	ListPRComments(org, repo string, number int) ([]sdk.PullRequestComments, error)
	DeletePRComment(org, repo string, ID int) error
	CreatePRComment(org, repo string, number int, comment string) error
	AddPRLabel(org, repo string, number int, label string) error
	RemovePRLabel(org, repo string, number int, label string) error

	AssignPR(owner, repo string, number int, logins []string) error
	UnassignPR(owner, repo string, number int, logins []string) error
	AssignGiteeIssue(org, repo string, number string, login string) error
	UnassignGiteeIssue(org, repo string, number string, login string) error
	CreateGiteeIssueComment(org, repo string, number string, comment string) error

	IsCollaborator(owner, repo, login string) (bool, error)
	GetGiteePullRequest(org, repo string, number int) (sdk.PullRequest, error)
	GetSingleCommit(org, repo, SHA string) (github.SingleCommit, error)
}
