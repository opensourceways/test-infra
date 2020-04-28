package approve

import (
	sdk "gitee.com/openeuler/go-gitee/gitee"
	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/github"
)

type giteeClient interface {
	GetPullRequestChanges(org, repo string, number int) ([]github.PullRequestChange, error)
	GetPRLabels(org, repo string, number int) ([]sdk.Label, error)
	ListPRComments(org, repo string, number int) ([]sdk.PullRequestComments, error)
	DeletePRComment(org, repo string, ID int) error
	CreatePRComment(org, repo string, number int, comment string) error
	BotName() (string, error)
	AddPRLabel(org, repo string, number int, label string) error
	RemovePRLabel(org, repo string, number int, label string) error
}

var _ githubClient = (*ghclient)(nil)

type ghclient struct {
	giteeClient
}

func (c *ghclient) GetIssueLabels(org, repo string, number int) ([]github.Label, error) {
	var r []github.Label

	v, err := c.GetPRLabels(org, repo, number)
	if err != nil {
		return r, err
	}

	for _, i := range v {
		r = append(r, github.Label{Name: i.Name})
	}
	return r, nil
}

func (c *ghclient) ListIssueComments(org, repo string, number int) ([]github.IssueComment, error) {
	var r []github.IssueComment

	v, err := c.ListPRComments(org, repo, number)
	if err != nil {
		return r, err
	}

	for _, i := range v {
		r = append(r, gitee.ConvertGiteePRComment(i))
	}
	return r, nil
}

func (c *ghclient) DeleteComment(org, repo string, id int) error {
	return c.DeletePRComment(org, repo, id)
}

func (c *ghclient) CreateComment(org, repo string, number int, comment string) error {
	return c.CreatePRComment(org, repo, number, comment)
}

func (c *ghclient) AddLabel(org, repo string, number int, label string) error {
	return c.AddPRLabel(org, repo, number, label)
}

func (c *ghclient) RemoveLabel(org, repo string, number int, label string) error {
	return c.RemovePRLabel(org, repo, number, label)
}

func (c *ghclient) ListPullRequestComments(org, repo string, number int) ([]github.ReviewComment, error) {
	return []github.ReviewComment{}, nil
}

func (c *ghclient) ListReviews(org, repo string, number int) ([]github.Review, error) {
	return []github.Review{}, nil
}

func (c *ghclient) ListIssueEvents(org, repo string, num int) ([]github.ListedIssueEvent, error) {
	return []github.ListedIssueEvent{}, nil
}

func (c *ghclient) GetPullRequest(org, repo string, number int) (*github.PullRequest, error) {
	return nil, nil
}
