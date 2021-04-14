package lifecycle

import (
	"regexp"
	"strings"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	prowConfig "k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/labels"
	"k8s.io/test-infra/prow/pluginhelp"
)

var (
	lifecycleLabels = []string{labels.LifecycleActive, labels.LifecycleFrozen, labels.LifecycleStale, labels.LifecycleRotten}
	lifecycleRe     = regexp.MustCompile(`(?mi)^/(remove-)?lifecycle (active|frozen|stale|rotten)\s*$`)
)

type lifecycle struct {
	fGpc plugins.GetPluginConfig
	gec  gitee.Client
}

type noteEventAction interface {
	GetLabels() ([]sdk.Label, error)
	removeLabel(lb string) error
	addLabel(lb string) error
}


type lifecycleClient interface {
	AddPRLabel(org, repo string, number int, label string) error
	RemovePRLabel(org, repo string, number int, label string) error
	GetPRLabels(org, repo string, number int) ([]sdk.Label, error)
	AddIssueLabel(org, repo, number, label string) error
	RemoveIssueLabel(org, repo, number, label string) error
	GetIssueLabels(org, repo, number string) ([]sdk.Label, error)
	BotName() (string, error)
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
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/[remove-]lifecycle <frozen|stale|rotten>",
		Description: "Flags an issue or PR as frozen/stale/rotten",
		Featured:    false,
		WhoCanUse:   "Anyone can trigger this command,an exception that /lifecycle <stale|rotten> only trigger by bot.",
		Examples:    []string{"/lifecycle frozen", "/remove-lifecycle stale"},
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

	return handle(l.gec, log, e)
}

func isPr(et string) bool {
	return et == "PullRequest"
}

func handle(gec lifecycleClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	var ac noteEventAction
	if isPr(*(e.NoteableType)) {
		ac = &prAction{e, gec}
	} else {
		ac = &issueAction{e, gec}
	}
	botName, err := gec.BotName()
	if err != nil {
		return err
	}
	for _, mat := range lifecycleRe.FindAllStringSubmatch(e.Comment.Body, -1) {
		if !canHandOne(mat[1], mat[2], botName, e.Comment.User.Login) {
			continue
		}
		if err := handleOne(ac, log, mat); err != nil {
			return err
		}
	}
	return nil
}

func handleOne(gc noteEventAction, log *logrus.Entry, mat []string) error {
	remove := mat[1] != ""
	cmd := mat[2]
	lbl := "lifecycle/" + cmd
	lbs, err := gc.GetLabels()
	if err != nil {
		log.WithError(err).Errorf("Failed to get labels.")
	}
	if hasLabel(lbl, lbs) && remove {
		return gc.removeLabel(lbl)
	}
	if !hasLabel(lbl, lbs) && !remove {
		for _, label := range lifecycleLabels {
			if label != lbl && hasLabel(label, lbs) {
				if err := gc.removeLabel(label); err != nil {
					log.WithError(err).Errorf("Gitee failed to remove the following label: %s", label)
				}
			}
		}
		if err := gc.addLabel(lbl); err != nil {
			log.WithError(err).Errorf("Gitee failed to add the following label: %s", lbl)
		}
	}
	return nil
}

func canHandOne(action, cmd, botName, userName string) bool {
	if action != "" {
		return true
	}
	if cmd != "stale" && cmd != "rotten" {
		return true
	}
	if botName == userName {
		return true
	}
	return false
}

type issueAction struct {
	e   *sdk.NoteEvent
	ghc lifecycleClient
}

func (ia *issueAction) GetLabels() ([]sdk.Label, error) {
	return ia.ghc.GetIssueLabels(ia.e.Repository.Namespace, ia.e.Repository.Path, ia.e.Issue.Number)
}

func (ia *issueAction) removeLabel(lb string) error {
	return ia.ghc.RemoveIssueLabel(ia.e.Repository.Namespace, ia.e.Repository.Path, ia.e.Issue.Number, lb)
}

func (ia *issueAction) addLabel(lb string) error {
	return ia.ghc.AddIssueLabel(ia.e.Repository.Namespace, ia.e.Repository.Path, ia.e.Issue.Number, lb)
}

type prAction issueAction

func (pa *prAction) GetLabels() ([]sdk.Label, error) {
	return pa.ghc.GetPRLabels(pa.e.Repository.Namespace, pa.e.Repository.Path, int(pa.e.PullRequest.Number))
}

func (pa *prAction) removeLabel(lb string) error {
	return pa.ghc.RemovePRLabel(pa.e.Repository.Namespace, pa.e.Repository.Path, int(pa.e.PullRequest.Number), lb)
}

func (pa *prAction) addLabel(lb string) error {
	return pa.ghc.AddPRLabel(pa.e.Repository.Namespace, pa.e.Repository.Path, int(pa.e.PullRequest.Number), lb)
}

func hasLabel(label string, issueLabels []sdk.Label) bool {
	for _, l := range issueLabels {
		if strings.ToLower(l.Name) == strings.ToLower(label) {
			return true
		}
	}
	return false
}
