// Package hold contains a plugin which will allow users to label their
// own pull requests as not ready or ready for merge of gitee platform. 
//The submit queue will honor the label to ensure pull requests do not 
//merge when it is applied.
package hold

import (
	"regexp"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/labels"
	"k8s.io/test-infra/prow/pluginhelp"
)

const pluginName = "hold"

const (
	misMatch = iota
	rmLabel
	addLabel
)

var (
	labelRe       = regexp.MustCompile(`(?mi)^/hold(\s.*)?$`)
	labelCancelRe = regexp.MustCompile(`(?mi)^/(hold\s+cancel|unhold)\s*$`)
)

type client interface {
	AddPRLabel(org, repo string, number int, label string) error
	RemovePRLabel(org, repo string, number int, label string) error
	GetPRLabels(org, repo string, number int) ([]sdk.Label, error)
}

type hold struct {
	ghc client
	fpc plugins.GetPluginConfig
}

func (hold *hold) HelpProvider(_ []config.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The hold plugin allows anyone to add or remove the '" + labels.Hold + "' Label from a pull request in order to temporarily prevent the PR from merging without withholding approval.",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/[un]hold [cancel]",
		Description: "Adds or removes the `" + labels.Hold + "` Label which is used to indicate that the PR should not be automatically merged.",
		Featured:    false,
		WhoCanUse:   "Anyone can use the /hold command to add or remove the '" + labels.Hold + "' Label.",
		Examples:    []string{"/hold", "/hold cancel", "/unhold"},
	})
	return pluginHelp, nil
}

func (hold *hold) PluginName() string {
	return pluginName
}

func (hold *hold) NewPluginConfig() plugins.PluginConfig {
	return nil
}

func (hold *hold) RegisterEventHandler(p plugins.Plugins) {
	p.RegisterNoteEventHandler(hold.PluginName(), hold.handleNoteEvent)
}

func (hold *hold) handleNoteEvent(e *sdk.NoteEvent, _ *logrus.Entry) error {
	ew := gitee.NewNoteEventWrapper(e)
	if !ew.IsCreatingCommentEvent() || !ew.IsPullRequest() {
		return nil
	}

	comment := ew.GetComment()
	action := misMatch
	if labelCancelRe.MatchString(comment) {
		action = rmLabel
	} else if labelRe.MatchString(comment) {
		action = addLabel
	}
	if action != misMatch {
		return hold.handle(gitee.NewPRNoteEvent(ew.NoteEvent), action)
	}
	return nil
}

func (hold *hold) handle(e gitee.PRNoteEvent, action int) error {
	org, repo := e.GetOrgRep()
	number := e.GetPRNumber()
	prLabels, err := hold.ghc.GetPRLabels(org, repo, number)
	if err != nil {
		return err
	}
	hasLabel := plugins.HasLabel(labels.Hold, prLabels)
	if hasLabel && action == rmLabel {
		return hold.ghc.RemovePRLabel(org, repo, number, labels.Hold)
	}
	if !hasLabel && action == addLabel {
		return hold.ghc.AddPRLabel(org, repo, number, labels.Hold)
	}
	return nil
}

//NewHold create a hold plugin
func NewHold(f plugins.GetPluginConfig, ghc gitee.Client) plugins.Plugin {
	return &hold{ghc: ghc, fpc: f}
}
