package gitee

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

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

func (c *client) GetPullRequest(org, repo string, number int) (*github.PullRequest, error) {
	return nil, nil
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

// actually this method return the labels of pull request not issue
func (c *client) GetIssueLabels(org, repo string, number int) ([]github.Label, error) {
	var r []github.Label

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

		for _, i := range ls {
			r = append(r, github.Label{Name: i.Name})
		}
	}

	return r, nil
}

func (c *client) ListIssueComments(org, repo string, number int) ([]github.IssueComment, error) {
	var r []github.IssueComment

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

		for _, i := range cs {
			ct, _ := time.Parse(time.RFC3339, i.CreatedAt)
			ut, _ := time.Parse(time.RFC3339, i.UpdatedAt)

			cm := github.IssueComment{
				ID:        int(i.Id),
				Body:      i.Body,
				User:      github.User{Login: i.User.Login},
				HTMLURL:   i.HtmlUrl,
				CreatedAt: ct,
				UpdatedAt: ut,
			}
			r = append(r, cm)
		}
	}

	return r, nil
}

func (c *client) ListReviews(org, repo string, number int) ([]github.Review, error) {
	return []github.Review{}, nil
}

func (c *client) ListPullRequestComments(org, repo string, number int) ([]github.ReviewComment, error) {
	return []github.ReviewComment{}, nil
}

func (c *client) DeleteComment(org, repo string, ID int) error {
	_, err := c.ac.PullRequestsApi.DeleteV5ReposOwnerRepoPullsCommentsId(
		context.Background(), org, repo, int32(ID), nil)
	return err
}

func (c *client) CreateComment(org, repo string, number int, comment string) error {
	opt := sdk.PullRequestCommentPostParam{Body: comment}
	_, _, err := c.ac.PullRequestsApi.PostV5ReposOwnerRepoPullsNumberComments(
		context.Background(), org, repo, int32(number), opt)
	return err
}

func (c *client) AddLabel(org, repo string, number int, label string) error {
	opt := sdk.PullRequestLabelPostParam{Body: []string{label}}
	_, _, err := c.ac.PullRequestsApi.PostV5ReposOwnerRepoPullsNumberLabels(
		context.Background(), org, repo, int32(number), opt)
	return err
}

func (c *client) RemoveLabel(org, repo string, number int, label string) error {
	_, err := c.ac.PullRequestsApi.DeleteV5ReposOwnerRepoPullsLabel(
		context.Background(), org, repo, int32(number), label, nil)
	return err
}

func (c *client) ListIssueEvents(org, repo string, num int) ([]github.ListedIssueEvent, error) {
	return []github.ListedIssueEvent{}, nil
}
