package cla

import sdk "gitee.com/openeuler/go-gitee/gitee"

type giteeClient interface {
	AddPRLabel(owner, repo string, number int, label string) error
	RemovePRLabel(owner, repo string, number int, label string) error
	CreatePRComment(owner, repo string, number int, comment string) error
	GetPRCommits(org, repo string, number int) ([]sdk.PullRequestCommits, error)
}

type ghclient struct {
	giteeClient
}

func (c *ghclient) AddLabel(org, repo string, number int, label string) error {
	return c.AddPRLabel(org, repo, number, label)
}

func (c *ghclient) CreateComment(owner, repo string, number int, comment string) error {
	return c.CreatePRComment(owner, repo, number, comment)
}

func (c *ghclient) RemoveLabel(org, repo string, number int, label string) error {
	return c.RemovePRLabel(org, repo, number, label)
}

func (c *ghclient) GetCommits(org, repo string, number int) ([]sdk.PullRequestCommits, error) {
	return c.GetPRCommits(org, repo, number)
}
