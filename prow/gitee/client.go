package gitee

import (
	"context"
	"fmt"
	"strings"
	"sync"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"
	"golang.org/x/oauth2"
	"k8s.io/test-infra/prow/github"
)

var _ Client = (*client)(nil)

type client struct {
	ac *sdk.APIClient

	mut      sync.Mutex // protects botName and email
	userData *sdk.User
}

func NewClient(getToken func() []byte) Client {
	token := string(getToken())

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	conf := sdk.NewConfiguration()
	conf.HTTPClient = oauth2.NewClient(context.Background(), ts)

	c := sdk.NewAPIClient(conf)
	return &client{ac: c}
}

// BotName returns the login of the authenticated identity.
func (c *client) BotName() (string, error) {
	err := c.getUserData()
	if err != nil {
		return "", err
	}

	return c.userData.Login, nil
}

func (c *client) Email() (string, error) {
	err := c.getUserData()
	if err != nil {
		return "", err
	}

	return c.userData.Email, nil
}

func (c *client) BotUser() (*github.User, error) {
	err := c.getUserData()
	if err != nil {
		return nil, err
	}

	d := c.userData
	u := github.User{
		Login: d.Login,
		Name:  d.Name,
		Email: d.Email,
		ID:    int(d.Id),
	}

	return &u, nil
}

func (c *client) CreatePullRequest(org, repo, title, body, head, base string, canModify bool) (sdk.PullRequest, error) {
	opts := sdk.CreatePullRequestParam{
		Title:             title,
		Head:              head,
		Base:              base,
		Body:              body,
		PruneSourceBranch: true,
	}

	pr, _, err := c.ac.PullRequestsApi.PostV5ReposOwnerRepoPulls(
		context.Background(), org, repo, opts)

	return pr, err
}

func (c *client) GetPullRequests(org, repo, state, head, base string) ([]sdk.PullRequest, error) {
	opts := sdk.GetV5ReposOwnerRepoPullsOpts{
		State: optional.NewString(state),
		Head:  optional.NewString(head),
		Base:  optional.NewString(base),
	}
	prs, _, err := c.ac.PullRequestsApi.GetV5ReposOwnerRepoPulls(context.Background(), org, repo, &opts)
	return prs, err
}

func (c *client) UpdatePullRequest(org, repo string, number int32, title, body, state, labels string) (sdk.PullRequest, error) {
	opts := sdk.PullRequestUpdateParam{
		Title:  title,
		Body:   body,
		State:  state,
		Labels: labels,
	}
	pr, _, err := c.ac.PullRequestsApi.PatchV5ReposOwnerRepoPullsNumber(context.Background(), org, repo, number, opts)
	return pr, err
}

func (c *client) getUserData() error {
	if c.userData == nil {
		c.mut.Lock()
		defer c.mut.Unlock()

		if c.userData == nil {
			u, _, err := c.ac.UsersApi.GetV5User(context.Background(), nil)
			if err != nil {
				return fmt.Errorf("fetching bot name from Gitee: %v", err)
			}
			c.userData = &u
		}
	}
	return nil
}

func (c *client) ListCollaborators(org, repo string) ([]github.User, error) {
	cs, _, err := c.ac.RepositoriesApi.GetV5ReposOwnerRepoCollaborators(context.Background(), org, repo, nil)
	if err != nil {
		return nil, err
	}
	var r []github.User
	for _, i := range cs {
		c := github.User{
			Login: i.Login,
		}
		r = append(r, c)
	}
	return r, nil
}

func (c *client) GetRef(org, repo, ref string) (string, error) {
	branch := strings.TrimPrefix(ref, "heads/")
	b, _, err := c.ac.RepositoriesApi.GetV5ReposOwnerRepoBranchesBranch(context.Background(), org, repo, branch, nil)
	if err != nil {
		return "", err
	}

	return b.Commit.Sha, nil
}

func (c *client) GetPullRequestChanges(org, repo string, number int) ([]github.PullRequestChange, error) {
	fs, _, err := c.ac.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberFiles(
		context.Background(), org, repo, int32(number), nil)
	if err != nil {
		return nil, err
	}

	var r []github.PullRequestChange

	for _, f := range fs {
		r = append(r, github.PullRequestChange{Filename: f.Filename})
	}
	return r, nil
}

func (c *client) GetPRLabels(org, repo string, number int) ([]sdk.Label, error) {
	var r []sdk.Label

	p := int32(1)
	opt := sdk.GetV5ReposOwnerRepoPullsNumberLabelsOpts{}
	for {
		opt.Page = optional.NewInt32(p)
		ls, _, err := c.ac.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberLabels(
			context.Background(), org, repo, int32(number), &opt)
		if err != nil {
			return nil, err
		}

		if len(ls) == 0 {
			break
		}

		p += 1

		r = append(r, ls...)
	}

	return r, nil
}

func (c *client) ListPRComments(org, repo string, number int) ([]sdk.PullRequestComments, error) {
	var r []sdk.PullRequestComments

	p := int32(1)
	opt := sdk.GetV5ReposOwnerRepoPullsNumberCommentsOpts{}
	for {
		opt.Page = optional.NewInt32(p)
		cs, _, err := c.ac.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberComments(
			context.Background(), org, repo, int32(number), &opt)
		if err != nil {
			return nil, err
		}

		if len(cs) == 0 {
			break
		}

		p += 1
		r = append(r, cs...)
	}

	return r, nil
}

func (c *client) DeletePRComment(org, repo string, ID int) error {
	_, err := c.ac.PullRequestsApi.DeleteV5ReposOwnerRepoPullsCommentsId(
		context.Background(), org, repo, int32(ID), nil)
	return err
}

func (c *client) CreatePRComment(org, repo string, number int, comment string) error {
	opt := sdk.PullRequestCommentPostParam{Body: comment}
	_, _, err := c.ac.PullRequestsApi.PostV5ReposOwnerRepoPullsNumberComments(
		context.Background(), org, repo, int32(number), opt)
	return err
}

func (c *client) AddPRLabel(org, repo string, number int, label string) error {
	opt := sdk.PullRequestLabelPostParam{Body: []string{label}}
	_, _, err := c.ac.PullRequestsApi.PostV5ReposOwnerRepoPullsNumberLabels(
		context.Background(), org, repo, int32(number), opt)
	return err
}

func (c *client) RemovePRLabel(org, repo string, number int, label string) error {
	_, err := c.ac.PullRequestsApi.DeleteV5ReposOwnerRepoPullsLabel(
		context.Background(), org, repo, int32(number), label, nil)
	return err
}

func (c *client) AssignPR(org, repo string, number int, logins []string) error {
	opt := sdk.PullRequestAssigneePostParam{Assignees: strings.Join(logins, ",")}

	_, _, err := c.ac.PullRequestsApi.PostV5ReposOwnerRepoPullsNumberAssignees(
		context.Background(), org, repo, int32(number), opt)
	return err
}

func (c *client) UnassignPR(org, repo string, number int, logins []string) error {
	_, _, err := c.ac.PullRequestsApi.DeleteV5ReposOwnerRepoPullsNumberAssignees(
		context.Background(), org, repo, int32(number), strings.Join(logins, ","), nil)
	return err
}

func (c *client) AssignGiteeIssue(org, repo string, number string, login string) error {
	opt := sdk.IssueUpdateParam{
		Repo:     repo,
		Assignee: login,
	}
	_, v, err := c.ac.IssuesApi.PatchV5ReposOwnerIssuesNumber(
		context.Background(), org, number, opt)

	if err != nil {
		if v.StatusCode == 403 {
			return ErrorForbidden{err: err.Error()}
		}
	}
	return err
}

func (c *client) UnassignGiteeIssue(org, repo string, number string, login string) error {
	return c.AssignGiteeIssue(org, repo, number, " ")
}

func (c *client) CreateGiteeIssueComment(org, repo string, number string, comment string) error {
	opt := sdk.IssueCommentPostParam{Body: comment}
	_, _, err := c.ac.IssuesApi.PostV5ReposOwnerRepoIssuesNumberComments(
		context.Background(), org, repo, number, opt)
	return err
}

func (c *client) IsCollaborator(owner, repo, login string) (bool, error) {
	v, err := c.ac.RepositoriesApi.GetV5ReposOwnerRepoCollaboratorsUsername(
		context.Background(), owner, repo, login, nil)
	if err != nil {
		if v.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
