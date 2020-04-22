package approve

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	prowConfig "k8s.io/test-infra/prow/config"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pluginhelp"
	originp "k8s.io/test-infra/prow/plugins"
	origina "k8s.io/test-infra/prow/plugins/approve"
	"k8s.io/test-infra/prow/repoowners"
)

const (
	approveCommand = "APPROVE"
	lgtmCommand    = "LGTM"
)

type approve struct {
	getPluginConfig plugins.GetPluginConfig
	ghc             githubClient
	oc              ownersClient
}

type githubClient interface {
	GetPullRequest(org, repo string, number int) (*github.PullRequest, error)
	GetPullRequestChanges(org, repo string, number int) ([]github.PullRequestChange, error)
	GetIssueLabels(org, repo string, number int) ([]github.Label, error)
	ListIssueComments(org, repo string, number int) ([]github.IssueComment, error)
	ListReviews(org, repo string, number int) ([]github.Review, error)
	ListPullRequestComments(org, repo string, number int) ([]github.ReviewComment, error)
	DeleteComment(org, repo string, ID int) error
	CreateComment(org, repo string, number int, comment string) error
	BotName() (string, error)
	AddLabel(org, repo string, number int, label string) error
	RemoveLabel(org, repo string, number int, label string) error
	ListIssueEvents(org, repo string, num int) ([]github.ListedIssueEvent, error)
}

type ownersClient interface {
	LoadRepoOwners(org, repo, base string) (repoowners.RepoOwner, error)
}

func NewApprove(f plugins.GetPluginConfig, ghc githubClient, oc ownersClient) plugins.Plugin {
	return &approve{
		getPluginConfig: f,
		ghc:             ghc,
		oc:              oc,
	}
}

func (a *approve) HelpProvider(enabledRepos []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	c, err := a.pluginConfig()
	if err != nil {
		return nil, err
	}

	c1 := originp.Configuration{Approve: c.Approve}

	h, ok := originp.HelpProviders()[a.PluginName()]
	if !ok {
		return nil, fmt.Errorf("can't find the approve's original helper method")
	}

	return h(&c1, enabledRepos)
}

func (a *approve) PluginName() string {
	return origina.PluginName
}

func (a *approve) NewPluginConfig() plugins.PluginConfig {
	return &configuration{}
}

func (a *approve) RegisterEventHandler(p plugins.Plugins) {
	name := a.PluginName()
	p.RegisterNoteEventHandler(name, a.handleNoteEvent)
	p.RegisterPullRequestHandler(name, a.handlePullRequestEvent)
}

func isApprovalCommand(botName, author, comment string, lgtmActsAsApprove bool) bool {
	if author == botName {
		return false
	}

	commandRegex := regexp.MustCompile(`(?m)^/([^\s]+)[\t ]*([^\n\r]*)`)

	for _, match := range commandRegex.FindAllStringSubmatch(comment, -1) {
		cmd := strings.ToUpper(match[1])
		if (cmd == lgtmCommand && lgtmActsAsApprove) || cmd == approveCommand {
			return true
		}
	}
	return false
}

func (a *approve) handleNoteEvent(e *gitee.NoteEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handleNoteEvent")
	}()

	if *(e.Action) != "comment" {
		log.Debug("Event is not a creation of a comment on an open PR, skipping.")
		return nil
	}

	botName, err := a.ghc.BotName()
	if err != nil {
		return err
	}

	org := e.Repository.Owner.Login
	repo := e.Repository.Name

	c, err := a.approveFor(org, repo)
	if err != nil {
		return err
	}

	if !isApprovalCommand(botName, e.Author.Name, e.Comment.Body, c.LgtmActsAsApprove) {
		log.Debug("Comment does not constitute approval, skipping event.")
		return nil
	}

	return a.handle(org, repo, e.PullRequest, log)
}

func (a *approve) handle(org, repo string, pr *gitee.PullRequestHook, log *logrus.Entry) error {
	c, err := a.approveFor(org, repo)
	if err != nil {
		return err
	}

	repoc, err := a.oc.LoadRepoOwners(org, repo, pr.Base.Ref)
	if err != nil {
		return err
	}

	assignees := make([]github.User, 0, len(pr.Assignees))
	for i, item := range pr.Assignees {
		assignees[i] = github.User{Login: item.Login}
	}

	return origina.Handle(
		log,
		a.ghc,
		repoc,
		getGiteeOption(),
		c,
		origina.NewState(org, repo, pr.Base.Ref, pr.Body, pr.User.Login, pr.HtmlUrl, int(pr.Number), assignees),
	)
}

func (a *approve) handlePullRequestEvent(e *gitee.PullRequestEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handlePullRequest")
	}()

	if *(e.State) != "open" {
		log.Debug("Pull request state is not open, skipping...")
		return nil
	}

	if *(e.Action) != "open" && *(e.Action) != "update" {
		log.Debug("Pull request event action cannot constitute approval, skipping...")
		return nil
	}

	return a.handle(e.Repository.Owner.Login, e.Repository.Name, e.PullRequest, log)
}

func getGiteeOption() prowConfig.GitHubOptions {
	s := "https://gitee.com"
	linkURL, _ := url.Parse(s)
	return prowConfig.GitHubOptions{LinkURLFromConfig: s, LinkURL: linkURL}
}

func (a *approve) pluginConfig() (*configuration, error) {
	c := a.getPluginConfig(a.PluginName())
	if c == nil {
		return nil, fmt.Errorf("can't find the approve's configuration")
	}

	c1, ok := c.(*configuration)
	if !ok {
		return nil, fmt.Errorf("can't convert to approve's configuration")
	}
	return c1, nil
}

func (a *approve) approveFor(org, repo string) (*originp.Approve, error) {
	c, err := a.pluginConfig()
	if err != nil {
		return nil, err
	}

	c1 := originp.Configuration{Approve: c.Approve}
	return c1.ApproveFor(org, repo), nil
}
