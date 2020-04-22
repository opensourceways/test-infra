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
	GetPullRequest(org, repo string, number int) (*github.PullRequest, error)
	GetPullRequestChanges(org, repo string, number int) ([]github.PullRequestChange, error)
	GetIssueLabels(org, repo string, number int) ([]github.Label, error)
	ListIssueComments(org, repo string, number int) ([]github.IssueComment, error)
	ListReviews(org, repo string, number int) ([]github.Review, error)
	ListPullRequestComments(org, repo string, number int) ([]github.ReviewComment, error)
	DeleteComment(org, repo string, ID int) error
	CreateComment(org, repo string, number int, comment string) error
	AddLabel(org, repo string, number int, label string) error
	RemoveLabel(org, repo string, number int, label string) error
	ListIssueEvents(org, repo string, num int) ([]github.ListedIssueEvent, error)
}
