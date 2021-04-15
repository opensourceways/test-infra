package label

import (
	"fmt"
	"regexp"
	"strings"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/prow/github"

	prowConfig "k8s.io/test-infra/prow/config"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/pluginhelp"
)

var (
	defaultLabels    = []string{"kind", "priority", "sig"}
	labelRegex       = regexp.MustCompile(`(?m)^/(kind|priority|sig)\s*(.*?)\s*$`)
	removeLabelRegex = regexp.MustCompile(`(?m)^/remove-(kind|priority|sig)\s*(.*?)\s*$`)
)

type giteeClient interface {
	GetRepoLabels(owner, repo string) ([]sdk.Label, error)
	GetIssueLabels(org, repo, number string) ([]sdk.Label, error)
	GetPRLabels(org, repo string, number int) ([]sdk.Label, error)

	AddIssueLabel(org, repo, number, label string) error
	RemoveIssueLabel(org, repo, number, label string) error

	AddMultiIssueLabel(org, repo, number string, label []string) error
	AddMultiPRLabel(org, repo string, number int, label []string) error
	RemovePRLabel(org, repo string, number int, label string) error

	CreatePRComment(org, repo string, number int, comment string) error
	CreateGiteeIssueComment(org, repo string, number string, comment string) error
}

type label struct {
	ghc             giteeClient
	getPluginConfig plugins.GetPluginConfig
}

func NewLabel(f plugins.GetPluginConfig, gec giteeClient) plugins.Plugin {
	return &label{ghc: gec, getPluginConfig: f}
}

func (l *label) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	var labels []string
	labels = append(labels, defaultLabels...)
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The label plugin provides commands that add or remove certain types of labels. Labels of the following types can be manipulated: 'kind/*', 'priority/*', 'sig/*'.",
		Config: map[string]string{
			"": configString(labels),
		},
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/[remove-](kind|priority|sig|label) <target>",
		Description: "Applies or removes a label from one of the recognized types of labels.",
		Featured:    false,
		WhoCanUse:   "Anyone can trigger this command on a PR.",
		Examples:    []string{"/kind bug", "/sig testing", "/priority high"},
	})
	return pluginHelp, nil
}

func (l *label) PluginName() string {
	return "label"
}

func (l *label) NewPluginConfig() plugins.PluginConfig {
	return &configuration{}
}

func (l *label) RegisterEventHandler(p plugins.Plugins) {
	p.RegisterNoteEventHandler(l.PluginName(), l.handleNoteEvent)
	p.RegisterPullRequestHandler(l.PluginName(), l.handlePullRequestEvent)
}

func (l *label) getLabelCfg() (*configuration, error) {
	cfg := l.getPluginConfig(l.PluginName())
	if cfg == nil {
		return nil, fmt.Errorf("can't find the configuration")
	}
	lCfg, ok := cfg.(*configuration)
	if !ok {
		return nil, fmt.Errorf("can't convert to configuration")
	}
	return lCfg, nil
}

func (l *label) orgRepoCfg(org, repo string) (*labelCfg, error) {
	cfg, err := l.getLabelCfg()
	if err != nil {
		return nil, err
	}
	labelCfg := cfg.LabelFor(org, repo)
	if labelCfg == nil {
		return nil, fmt.Errorf("no label plugin config for this repo:%s/%s", org, repo)
	}
	return labelCfg, nil
}

func (l *label) handleNoteEvent(e *sdk.NoteEvent, log *logrus.Entry) error {
	if (*e.Action) != "comment" {
		log.Debug("Event is not a creation of a comment, skipping.")
		return nil
	}
	var action noteEventAction
	switch *e.NoteableType {
	case "PullRequest":
		action = &PRNoteAction{event: e, client: l.ghc}
	case "Issue":
		action = &IssueNoteAction{event: e, client: l.ghc}
	default:
		log.Debug("not support note type")
		return nil
	}

	return l.handleGenericCommentEvent(e, log, action)
}

func (l *label) handlePullRequestEvent(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	if e == nil {
		return fmt.Errorf("the event payload is empty")
	}
	tp := plugins.ConvertPullRequestAction(e)
	if tp == github.PullRequestActionSynchronize {
		return l.handleClearLabel(e, log)
	}

	return nil
}

func (l *label) handleGenericCommentEvent(e *sdk.NoteEvent, log *logrus.Entry, action noteEventAction) error {
	body := *e.Note
	labelMatches := labelRegex.FindAllStringSubmatch(body, -1)
	removeLabelMatches := removeLabelRegex.FindAllStringSubmatch(body, -1)
	if len(labelMatches) == 0 && len(removeLabelMatches) == 0 {
		return nil
	}

	org, repo, err := plugins.GetOwnerAndRepoByEvent(e)
	if err != nil {
		return err
	}
	repoLabels, err := l.ghc.GetRepoLabels(org, repo)
	if err != nil {
		return err
	}
	labels, err := action.getAllLabels()
	if err != nil {
		return err
	}

	repoLabelsExisting := labelsTransformMap(repoLabels)

	issueLabels := labelsTransformMap(labels)

	//remove labels
	removeMatchLabels(removeLabelMatches, issueLabels, action, log)

	//add labels
	noSuchLabelsInRepo := addMatchLabels(labelMatches, issueLabels, repoLabelsExisting, action, log)
	if len(noSuchLabelsInRepo) > 0 {
		msg := fmt.Sprintf(
			"The label(s) `%s` cannot be applied, because the repository doesn't have them", strings.Join(noSuchLabelsInRepo, ", "))
		return action.addComment(msg)
	}
	return nil
}

func removeMatchLabels(match [][]string, labels map[string]string, action noteEventAction, log *logrus.Entry) {
	if len(match) == 0 {
		return
	}
	labelsToRemove := getLabelsFromREMatches(match)

	// Remove labels
	for _, labelToRemove := range labelsToRemove {
		if label, ok := labels[labelToRemove]; ok {
			if err := action.removeLabel(label); err != nil {
				log.WithError(err).Errorf("Gitee failed to add the following label: %s", label)
			}
		}
	}
}

func addMatchLabels(matches [][]string, currentLabels, repoLabels map[string]string, action noteEventAction, log *logrus.Entry) []string {
	if len(matches) == 0 {
		return nil
	}

	labelsToAdd := getLabelsFromREMatches(matches)

	var noSuchLabelsInRepo []string
	// Add labels
	var canAddLabel []string
	for _, labelToAdd := range labelsToAdd {
		if _, ok := currentLabels[labelToAdd]; ok {
			continue
		}

		if label, ok := repoLabels[labelToAdd]; !ok {
			noSuchLabelsInRepo = append(noSuchLabelsInRepo, labelToAdd)
		} else {
			canAddLabel = append(canAddLabel, label)
		}
	}

	if len(canAddLabel) > 0 {
		if err := action.addLabel(canAddLabel); err != nil {
			log.WithError(err).Errorf("Gitee failed to add the following label: %s", strings.Join(canAddLabel, ","))
		}
	}

	return noSuchLabelsInRepo
}

func configString(labels []string) string {
	var formattedLabels []string
	for _, label := range labels {
		formattedLabels = append(formattedLabels, fmt.Sprintf(`"%s/*"`, label))
	}
	if len(formattedLabels) > 0 {
		return fmt.Sprintf("The label plugin will work on %s and %s labels.",
			strings.Join(formattedLabels[:len(formattedLabels)-1], ", "), formattedLabels[len(formattedLabels)-1])
	}
	return ""
}

// Get Labels from Regexp matches
func getLabelsFromREMatches(matches [][]string) (labels []string) {
	for _, match := range matches {
		for _, label := range strings.Split(match[0], " ")[1:] {
			label = strings.ToLower(match[1] + "/" + strings.TrimSpace(label))
			labels = append(labels, label)
		}
	}
	return
}

func labelsTransformMap(labels []sdk.Label) map[string]string {
	lm := make(map[string]string, len(labels))
	for _, v := range labels {
		k := strings.ToLower(v.Name)
		lm[k] = v.Name
	}
	return lm
}
