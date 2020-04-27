package lgtm

import (
	"sort"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/github"
)

type giteeClient interface {
	ListCollaborators(org, repo string) ([]github.User, error)
	AssignPR(owner, repo string, number int, logins []string) error
	IsCollaborator(owner, repo, login string) (bool, error)
	AddPRLabel(owner, repo string, number int, label string) error
	CreatePRComment(owner, repo string, number int, comment string) error
	RemovePRLabel(owner, repo string, number int, label string) error
	GetPRLabels(org, repo string, number int) ([]sdk.Label, error)
	GetGiteePullRequest(org, repo string, number int) (sdk.PullRequest, error)
	GetPullRequestChanges(org, repo string, number int) ([]github.PullRequestChange, error)
	ListPRComments(org, repo string, number int) ([]sdk.PullRequestComments, error)
	DeletePRComment(org, repo string, ID int) error
	BotName() (string, error)
	GetSingleCommit(org, repo, SHA string) (github.SingleCommit, error)
}

var _ githubClient = (*ghclient)(nil)

type ghclient struct {
	giteeClient
}

func (c *ghclient) AddLabel(org, repo string, number int, label string) error {
	return c.AddPRLabel(org, repo, number, label)
}

func (c *ghclient) AssignIssue(owner, repo string, number int, logins []string) error {
	return c.AssignPR(owner, repo, number, logins)
}

func (c *ghclient) CreateComment(owner, repo string, number int, comment string) error {
	return c.CreatePRComment(owner, repo, number, comment)
}

func (c *ghclient) DeleteComment(org, repo string, id int) error {
	return c.DeletePRComment(org, repo, id)
}

func (c *ghclient) RemoveLabel(org, repo string, number int, label string) error {
	return c.RemovePRLabel(org, repo, number, label)
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

	sort.SliceStable(r, func(i, j int) bool {
		return r[i].CreatedAt.Before(r[j].CreatedAt)
	})

	return r, nil
}

func (c *ghclient) IsMember(org, user string) (bool, error) {
	return false, nil
}

func (c *ghclient) ListTeams(org string) ([]github.Team, error) {
	return []github.Team{}, nil
}

func (c *ghclient) ListTeamMembers(id int, role string) ([]github.TeamMember, error) {
	return []github.TeamMember{}, nil
}

func (c *ghclient) GetPullRequest(org, repo string, number int) (*github.PullRequest, error) {
	v, err := c.GetGiteePullRequest(org, repo, number)
	if err != nil {
		return nil, err
	}

	return gitee.ConvertGiteePR(&v), nil
}
