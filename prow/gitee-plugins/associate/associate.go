package associate

import (
	"errors"
	sdk "gitee.com/openeuler/go-gitee/gitee"
	log "github.com/sirupsen/logrus"

	prowConfig "k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/pluginhelp"
)

type associate struct {
	getPluginConfig plugins.GetPluginConfig
	ghc             gitee.Client
}

//NewAssociate create a milestone plugin by config and gitee client
func NewAssociate(f plugins.GetPluginConfig, gec gitee.Client) plugins.Plugin {
	return &associate{
		getPluginConfig: f,
		ghc:             gec,
	}
}

func (m *associate) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The associate plugin is used to detect whether the issue is associated with a milestone and whether the PR is associated with an issue. ",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/check-milestone",
		Description: "Mandatory check whether the issue is set with milestone,remove or add miss/milestone label",
		Featured:    true,
		WhoCanUse:   "Anyone",
		Examples:    []string{"/check-milestone"},
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/check-issue",
		Description: "Mandatory check whether the PullRequest associated issue,remove or add miss-issue label",
		Featured:    true,
		WhoCanUse:   "Anyone",
		Examples:    []string{"/check-issue"},
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/remove-miss/issue",
		Description: "remove the miss/issue label",
		Featured:    true,
		WhoCanUse:   "Members of the project maintainer gitee team can use the '/remove-miss/issue' command.",
		Examples:    []string{"/remove-miss/issue"},
	})
	return pluginHelp, nil
}

func (m *associate) PluginName() string {
	return "associate"
}

func (m *associate) NewPluginConfig() plugins.PluginConfig {
	return nil
}

func (m *associate) RegisterEventHandler(p plugins.Plugins) {
	name := m.PluginName()
	p.RegisterIssueHandler(name, m.handleIssueEvent)
	p.RegisterNoteEventHandler(name, m.handleNoteEvent)
	p.RegisterPullRequestHandler(name, m.handlePREvent)
}

func (m *associate) handleIssueEvent(e *sdk.IssueEvent, log *log.Entry) error {
	if e == nil {
		return errors.New("event payload is nil")
	}
	act := *(e.Action)
	if act == "open" {
		return handleIssueCreate(m.ghc, e, log)
	}
	return handleIssueUpdate(m.ghc, e)
}

func (m *associate) handleNoteEvent(e *sdk.NoteEvent, log *log.Entry) error {
	if e == nil {
		return errors.New("event payload is nil")
	}

	if *(e.Action) != "comment" {
		log.Debug("Event is not a creation of a comment, skipping.")
		return nil
	}

	if *(e.NoteableType) == "Issue" {
		return handleIssueNoteEvent(m.ghc, e)
	}
	if *(e.NoteableType) == "PullRequest" {
		//handle pullrequest
		return handlePrComment(m.ghc, e)
	}
	return nil
}

func (m *associate) handlePREvent(e *sdk.PullRequestEvent, log *log.Entry) error {
	if err := handlePrCreate(m.ghc, e, log); err != nil {
		return err
	}
	return nil
}

func judgeHasLabel(labs []sdk.LabelHook, label string) bool {
	hasLabel := false
	for _, lab := range labs {
		if lab.Name == label {
			hasLabel = true
		}
	}
	return hasLabel
}
