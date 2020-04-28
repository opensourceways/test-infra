package assign

import (
	"fmt"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/github"
)

type giteeClient interface {
	ListCollaborators(org, repo string) ([]github.User, error)
	AssignPR(owner, repo string, number int, logins []string) error
	UnassignPR(owner, repo string, number int, logins []string) error
	CreatePRComment(owner, repo string, number int, comment string) error
	AssignGiteeIssue(org, repo string, number string, login string) error
	UnassignGiteeIssue(org, repo string, number string, login string) error
	CreateGiteeIssueComment(owner, repo string, number string, comment string) error
}

var _ githubClient = (*ghclient)(nil)

type ghclient struct {
	giteeClient
	e *sdk.NoteEvent
}

func (c *ghclient) ispr() bool {
	return *(c.e.NoteableType) == "PullRequest"
}

func (c *ghclient) issueNumber() string {
	return c.e.Issue.Number
}

func (c *ghclient) peopleCanAssignPRTo(owner, repo string, number int) (map[string]bool, error) {
	v, err := c.ListCollaborators(owner, repo)
	if err != nil {
		return nil, err
	}

	cs := map[string]bool{}
	for _, item := range v {
		cs[item.Login] = true
	}

	// TODO: maybe other people not only collaborators can assign pr

	return cs, nil
}

func (c *ghclient) assignPR(owner, repo string, number int, logins []string) error {
	cs, err := c.peopleCanAssignPRTo(owner, repo, number)
	if err != nil {
		return err
	}

	var toAdd []string
	var toExclude []string
	for _, i := range logins {
		if cs[i] {
			toAdd = append(toAdd, i)
		} else {
			toExclude = append(toExclude, i)
		}
	}

	if len(toAdd) > 0 {
		err = c.AssignPR(owner, repo, number, toAdd)
		if err != nil {
			return err
		}
	}

	if len(toExclude) > 0 {
		return github.MissingUsers{Users: toExclude}
	}
	return nil
}

func (c *ghclient) AssignIssue(owner, repo string, number int, logins []string) error {
	if c.ispr() {
		return c.assignPR(owner, repo, number, logins)
	}

	if len(logins) > 1 {
		return github.MissingUsers{Users: logins}
	}

	err := c.AssignGiteeIssue(owner, repo, c.issueNumber(), logins[0])
	if err != nil {
		if _, ok := err.(gitee.ErrorForbidden); ok {
			return github.MissingUsers{Users: logins}
		}
	}
	return err
}

func (c *ghclient) UnassignIssue(owner, repo string, number int, logins []string) error {
	if c.ispr() {
		return c.UnassignPR(owner, repo, number, logins)
	}

	if len(logins) > 1 {
		return fmt.Errorf("can't unassign more one persons from an issue at same time")
	}

	if c.e.Issue.Assignee != nil && c.e.Issue.Assignee.Login == logins[0] {
		return c.UnassignGiteeIssue(owner, repo, c.issueNumber(), logins[0])
	}
	return nil
}

func (c *ghclient) CreateComment(owner, repo string, number int, comment string) error {
	if c.ispr() {
		return c.CreatePRComment(owner, repo, number, comment)
	}

	return c.CreateGiteeIssueComment(owner, repo, c.issueNumber(), comment)
}

func (c *ghclient) RequestReview(org, repo string, number int, logins []string) error {
	return nil
}

func (c *ghclient) UnrequestReview(org, repo string, number int, logins []string) error {
	return nil
}
