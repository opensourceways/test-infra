package assign

import (
	"fmt"
	"strings"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	prowConfig "k8s.io/test-infra/prow/config"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pluginhelp"
	origina "k8s.io/test-infra/prow/plugins/assign"
)

type githubClient interface {
	AssignIssue(owner, repo string, number int, logins []string) error
	UnassignIssue(owner, repo string, number int, logins []string) error

	RequestReview(org, repo string, number int, logins []string) error
	UnrequestReview(org, repo string, number int, logins []string) error

	CreateComment(owner, repo string, number int, comment string) error
}

type assign struct {
	getPluginConfig plugins.GetPluginConfig
	gec             giteeClient
}

func NewAssign(f plugins.GetPluginConfig, gec giteeClient) plugins.Plugin {
	return &assign{
		getPluginConfig: f,
		gec:             gec,
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
	return origina.HandleAssign(ce, &ghclient{giteeClient: a.gec, e: e}, f, log)
}

func buildAssignPRFailureComment(a *assign, org, repo string) func(mu github.MissingUsers) string {

	return func(mu github.MissingUsers) string {
		v, err := a.gec.ListCollaborators(org, repo)
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

		v, err := a.gec.ListCollaborators(org, repo)
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
