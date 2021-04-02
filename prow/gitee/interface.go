package gitee

import (
	sdk "gitee.com/openeuler/go-gitee/gitee"
	"k8s.io/test-infra/prow/github"
)

// Client interface for Gitee API
type Client interface {
	github.UserClient

	CreatePullRequest(org, repo, title, body, head, base string, canModify bool) (sdk.PullRequest, error)
	GetPullRequests(org, repo string, opts ListPullRequestOpt) ([]sdk.PullRequest, error)
	UpdatePullRequest(org, repo string, number int32, param sdk.PullRequestUpdateParam) (sdk.PullRequest, error)

	ListCollaborators(org, repo string) ([]github.User, error)
	GetRef(org, repo, ref string) (string, error)
	GetPullRequestChanges(org, repo string, number int) ([]github.PullRequestChange, error)
	GetPRLabels(org, repo string, number int) ([]sdk.Label, error)
	ListPRComments(org, repo string, number int) ([]sdk.PullRequestComments, error)
	ListPrIssues(org, repo string, number int32) ([] sdk.Issue, error)
	DeletePRComment(org, repo string, ID int) error
	CreatePRComment(org, repo string, number int, comment string) error
	UpdatePRComment(org, repo string, commentID int, comment string) error
	AddPRLabel(org, repo string, number int, label string) error
	RemovePRLabel(org, repo string, number int, label string) error

	AssignPR(owner, repo string, number int, logins []string) error
	UnassignPR(owner, repo string, number int, logins []string) error
	GetPRCommits(org, repo string, number int) ([]sdk.PullRequestCommits, error)

	AssignGiteeIssue(org, repo string, number string, login string) error
	UnassignGiteeIssue(org, repo string, number string, login string) error
	CreateGiteeIssueComment(org, repo string, number string, comment string) error

	IsCollaborator(owner, repo, login string) (bool, error)
	IsMember(org, login string) (bool, error)
	GetGiteePullRequest(org, repo string, number int) (sdk.PullRequest, error)
	GetSingleCommit(org, repo, SHA string) (github.SingleCommit, error)
	GetGiteeRepo(org, repo string) (sdk.Project, error)
	MergePR(owner, repo string, number int, opt sdk.PullRequestMergePutParam) error

	GetRepos(org string) ([]sdk.Project, error)
	RemoveIssueLabel(org, repo, number, label string) error
	AddIssueLabel(org, repo, number, label string) error
	AddMultiIssueLabel(org, repo, number string, label []string) error

	ReplacePRAllLabels(owner, repo string, number int, labels []string) error
}

type ListPullRequestOpt struct {
	State           string
	Head            string
	Base            string
	Sort            string
	Direction       string
	MilestoneNumber int
	Labels          []string
}
