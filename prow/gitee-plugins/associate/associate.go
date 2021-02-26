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

//NewAssociate create a associate plugin by config and gitee client
func NewAssociate(f plugins.GetPluginConfig, gec gitee.Client) plugins.Plugin {
	return &associate{
		getPluginConfig: f,
		ghc:             gec,
	}
}

func (m *associate) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The associate plugin is used to detect whether the issue is associated with a milestone and whether the PR is associated with an issue.",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/check-milestone",
		Description: "Check whether the issue is set a milestone, remove or add needs-milestone label",
		Featured:    true,
		WhoCanUse:   "Anyone",
		Examples:    []string{"/check-milestone"},
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/check-issue",
		Description: "Check whether the Pull Request is associated with at least an issue, remove or add needs-issue label",
		Featured:    true,
		WhoCanUse:   "Anyone",
		Examples:    []string{"/check-issue"},
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/remove-needs-issue",
		Description: "remove the needs-issue label",
		Featured:    true,
		WhoCanUse:   "Members of the project can use the '/remove-needs-issue' command",
		Examples:    []string{"/remove-needs-issue"},
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
	return nil
}

func (m *associate) handleNoteEvent(e *sdk.NoteEvent, log *log.Entry) error {
	if e == nil {
		return errors.New("event payload is nil")
	}

	if *(e.Action) != "comment" {
		log.Debug("Event is not a creation of a comment, skipping.")
		return nil
	}
	switch *(e.NoteableType) {
	case "Issue":
		return handleIssueNoteEvent(m.ghc, e)
	case "PullRequest":
		return handlePrComment(m.ghc, e)
	default:
		return nil
	}
}

func (m *associate) handlePREvent(e *sdk.PullRequestEvent, log *log.Entry) error {
	if e == nil {
		return errors.New("pr event payload is nil")
	}
	if *(e.Action) == "open" {
		return handlePrCreate(m.ghc, e, log)
	}
	return nil
}

func hasLabel(labs []sdk.LabelHook, label string) bool {
	for _, lab := range labs {
		if lab.Name == label {
			return true
		}
	}
	return false
}
