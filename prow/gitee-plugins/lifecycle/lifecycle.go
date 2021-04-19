package lifecycle

import (
	"regexp"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/pluginhelp"
)

var (
	reopenRe = regexp.MustCompile(`(?mi)^/reopen\s*$`)
	closeRe  = regexp.MustCompile(`(?mi)^/close\s*$`)
)

type giteeClient interface {
	CreatePRComment(owner, repo string, number int, comment string) error
	CreateIssueComment(owner, repo string, number string, comment string) error
	IsCollaborator(owner, repo, login string) (bool, error)
	CloseIssue(owner, repo string, number string) error
	ClosePR(owner, repo string, number int) error
	ReopenIssue(owner, repo string, number string) error
}

type lifecycle struct {
	fGpc plugins.GetPluginConfig
	gec  giteeClient
}

func NewLifeCycle(f plugins.GetPluginConfig, gec gitee.Client) plugins.Plugin {
	return &lifecycle{
		fGpc: f,
		gec:  gec,
	}
}

func (l *lifecycle) HelpProvider(_ []config.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "Close an issue or PR",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/close",
		Featured:    false,
		Description: "Closes an issue or PullRequest.",
		Examples:    []string{"/close"},
		WhoCanUse:   "Authors and collaborators of the repository can trigger this command.",
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/reopen",
		Description: "Reopens an issue ",
		Featured:    false,
		WhoCanUse:   "Authors and collaborators of the repository can trigger this command.",
		Examples:    []string{"/reopen"},
	})
	return pluginHelp, nil
}

func (l *lifecycle) PluginName() string {
	return "lifecycle"
}

func (l *lifecycle) NewPluginConfig() plugins.PluginConfig {
	return nil
}

func (l *lifecycle) RegisterEventHandler(p plugins.Plugins) {
	name := l.PluginName()
	p.RegisterNoteEventHandler(name, l.handleNoteEvent)
}

func (l *lifecycle) handleNoteEvent(e *sdk.NoteEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handleNoteEvent")
	}()

	ne := gitee.NewNoteEventWrapper(e)

	if !ne.IsCreatingCommentEvent() {
		log.Debug("Event is not a creation of a comment for PR or issue, skipping.")
		return nil
	}

	if ne.IsPullRequest() {
		return closePullRequest(l.gec, log, e)
	}

	if ne.IsIssue() {
		return l.handleIssue(e, log)
	}

	return nil
}

func (l *lifecycle) handleIssue(e *sdk.NoteEvent, log *logrus.Entry) error {
	ne := gitee.NewIssueNoteEvent(e)

	if ne.IsIssueClosed() && reopenRe.MatchString(ne.GetComment()) {
		return reopenIssue(l.gec, log, ne)
	}

	if ne.IsIssueOpen() && closeRe.MatchString(ne.GetComment()) {
		return closeIssue(l.gec, log, ne)
	}
	return nil
}
