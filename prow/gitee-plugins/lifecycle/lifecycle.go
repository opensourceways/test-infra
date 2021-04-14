package lifecycle

import (
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	prowConfig "k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/pluginhelp"
)

type lifecycle struct {
	fGpc plugins.GetPluginConfig
	gec  gitee.Client
}

func NewLifeCycle(f plugins.GetPluginConfig, gec gitee.Client) plugins.Plugin {
	return &lifecycle{
		fGpc: f,
		gec:  gec,
	}
}

func (l *lifecycle) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "Close an issue or PR",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/close",
		Featured:    false,
		Description: "Closes an issue or PullRequest.",
		Examples:    []string{"/close"},
		WhoCanUse:   "Authors and collaborators on the repository can trigger this command.",
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/reopen",
		Description: "Reopens an issue ",
		Featured:    false,
		WhoCanUse:   "Authors and collaborators on the repository can trigger this command.",
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

	eType := *(e.NoteableType)
	if *(e.Action) != "comment" || (!isPr(eType) && eType != "Issue") {
		log.Debug("Event is not a creation of a comment for PR or issue, skipping.")
		return nil
	}
	if err := handleReopen(l.gec, log, e); err != nil {
		return err
	}
	if err := handleClose(l.gec, log, e); err != nil {
		return err
	}

	return nil
}

func isPr(et string) bool {
	return et == "PullRequest"
}
