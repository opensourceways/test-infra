package assign

import (
	"fmt"
	"strings"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	prowConfig "k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pluginhelp"
	origina "k8s.io/test-infra/prow/plugins/assign"
)

type githubClient interface {
	ListCollaborators(org, repo string) ([]github.User, error)
	AssignPR(owner, repo string, number int, logins []string) error
	UnassignPR(owner, repo string, number int, logins []string) error
	CreatePRComment(owner, repo string, number int, comment string) error
	AssignGiteeIssue(org, repo string, number string, login string) error
	UnassignGiteeIssue(org, repo string, number string, login string) error
	CreateGiteeIssueComment(owner, repo string, number string, comment string) error
}

type assign struct {
	getPluginConfig plugins.GetPluginConfig
	ghc             githubClient
}

type ghclient struct {
	githubClient
	e *sdk.NoteEvent
}

func (c *ghclient) ispr() bool {
	return *(c.e.NoteableType) == "PullRequest"
}

func (c *ghclient) issueNumber() string {
	return c.e.Issue.Number
}

func (c *ghclient) assignPR(owner, repo string, number int, logins []string) error {
	v, err := c.ListCollaborators(owner, repo)
	if err != nil {
		return err
	}

	cs := map[string]bool{}
	for _, i := range getCollaborators(v) {
		cs[i] = true
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

func NewAssign(f plugins.GetPluginConfig, ghc githubClient) plugins.Plugin {
	return &assign{
		getPluginConfig: f,
		ghc:             ghc,
	}
}

func (a *assign) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	return origina.HelpProvider(nil, nil)
}

func (a *assign) PluginName() string {
	return "assign"
}

func (a *assign) NewPluginConfig() plugins.PluginConfig {
	return &configuration{}
}

func (a *assign) RegisterEventHandler(p plugins.Plugins) {
	name := a.PluginName()
	p.RegisterNoteEventHandler(name, a.handleNoteEvent)
}

func (a *assign) handleNoteEvent(e *sdk.NoteEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handleNoteEvent")
	}()

	if *(e.Action) != "comment" {
		log.Debug("Event is not a creation of a comment, skipping.")
		return nil
	}

	var n int32
	isPR := true
	switch *(e.NoteableType) {
	case "PullRequest":
		n = e.PullRequest.Number
	case "Issue":
		isPR = false
	default:
		log.Debug("not supported note type")
		return nil
	}

	ce := github.GenericCommentEvent{
		Repo: github.Repo{
			Owner: github.User{Login: e.Repository.Owner.Login},
			Name:  e.Repository.Name,
		},
		Body:    e.Comment.Body,
		User:    github.User{Login: e.Comment.User.Login},
		Number:  int(n),
		HTMLURL: e.Comment.HtmlUrl,
		IsPR:    false,
	}

	var f func(mu github.MissingUsers) string
	if isPR {
		f = buildAssignPRFailureComment(a, ce.Repo.Owner.Login, ce.Repo.Name)
	} else {
		f = buildAssignIssueFailureComment(a, ce.Repo.Owner.Login, ce.Repo.Name)
	}
	return origina.HandleAssign(ce, &ghclient{githubClient: a.ghc, e: e}, f, log)
}

func buildAssignPRFailureComment(a *assign, org, repo string) func(mu github.MissingUsers) string {

	return func(mu github.MissingUsers) string {
		v, err := a.ghc.ListCollaborators(org, repo)
		if err == nil {
			v1 := getCollaborators(v)

			return fmt.Sprintf("Gitee didn't allow you to assign to: %s.\n\nChoose following members as assignees.\n- %s", strings.Join(mu.Users, ", "), strings.Join(v1, "\n- "))
		}

		return fmt.Sprintf("Gitee didn't allow you to assign to: %s.", strings.Join(mu.Users, ", "))
	}
}

func buildAssignIssueFailureComment(a *assign, org, repo string) func(mu github.MissingUsers) string {

	return func(mu github.MissingUsers) string {
		if len(mu.Users) > 1 {
			return "Can only assign one person to an issue."
		}

		v, err := a.ghc.ListCollaborators(org, repo)
		if err == nil {
			v1 := getCollaborators(v)

			return fmt.Sprintf("Gitee didn't allow you to assign to: %s.\n\nChoose one of following members as assignee.\n- %s", mu.Users[0], strings.Join(v1, "\n- "))
		}

		return fmt.Sprintf("Gitee didn't allow you to assign to: %s.", mu.Users[0])
	}
}

func getCollaborators(u []github.User) []string {
	r := make([]string, len(u))
	for i, item := range u {
		r[i] = item.Login
	}
	return r
}
