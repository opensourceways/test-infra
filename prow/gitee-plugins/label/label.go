package label

import (
	"fmt"
	"k8s.io/test-infra/prow/github"
	"regexp"
	"strings"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	prowConfig "k8s.io/test-infra/prow/config"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/pluginhelp"
)

var (
	defaultLabels          = []string{"kind", "priority", "sig"}
	labelRegex             = regexp.MustCompile(`(?m)^/(kind|priority|sig)\s*(.*?)\s*$`)
	removeLabelRegex       = regexp.MustCompile(`(?m)^/remove-(kind|priority|sig)\s*(.*?)\s*$`)
	customLabelRegex       = regexp.MustCompile(`(?m)^/label\s*(.*?)\s*$`)
	customRemoveLabelRegex = regexp.MustCompile(`(?m)^/remove-label\s*(.*?)\s*$`)
)

type giteeClient interface {
	GetRepoLabels(owner, repo string) ([]sdk.Label, error)
	GetIssueLabels(org, repo, number string) ([]sdk.Label, error)
	GetPRLabels(org, repo string, number int) ([]sdk.Label, error)

	AddIssueLabel(org, repo, number, label string) error
	RemoveIssueLabel(org, repo, number, label string) error

	AddPRLabel(org, repo string, number int, label string) error
	RemovePRLabel(org, repo string, number int, label string) error

	CreatePRComment(org, repo string, number int, comment string) error
	CreateGiteeIssueComment(org, repo string, number string, comment string) error

	UpdatePullRequest(org, repo string, number int32, title, body, state, labels string) (sdk.PullRequest, error)
	GetPullRequestOperateLogs(org, repo string, number int32) ([]sdk.OperateLog, error)
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
	if cfg, err := l.getLabelCfg(); err == nil {
		labels = append(labels, cfg.Label.AdditionalLabels...)
	}
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The label plugin provides commands that add or remove certain types of labels. Labels of the following types can be manipulated: 'kind/*', 'priority/*', 'sig/*'. More labels can be configured to be used via the /label command.",
		Config: map[string]string{
			"": configString(labels),
		},
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/[remove-](kind|priority|sig|label) <target>",
		Description: "Applies or removes a label from one of the recognized types of labels.",
		Featured:    false,
		WhoCanUse:   "Anyone can trigger this command on a PR.",
		Examples:    []string{"/kind bug", "/sig testing", "/label foo-bar-baz"},
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

func (l *label) handleNoteEvent(e *sdk.NoteEvent, log *logrus.Entry) error {
	if e == nil {
		return fmt.Errorf("the event payload is empty")
	}
	if (*e.Action) != "comment" {
		log.Debug("Event is not a creation of a comment, skipping.")
		return nil
	}
	isPr := false
	switch *e.NoteableType {
	case "PullRequest":
		isPr = true
	case "Issue":
		isPr = false
	default:
		log.Debug("not support note type")
		return nil
	}
	var action noteEventAction
	if isPr {
		action = &PRNoteAction{event: e, client: l}
	} else {
		action = &IssueNoteAction{event: e, client: l}
	}
	return l.handleGenericCommentEvent(e, log, action)
}

func (l *label) handlePullRequestEvent(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	if e == nil {
		return fmt.Errorf("the event payload is empty")
	}
	tp := plugins.ConvertPullRequestAction(e)
	switch tp {
	case github.PullRequestActionLabeled:
		return l.handleCheckLimitLabel(e, log)
	case github.PullRequestActionSynchronize:
		return l.handleClearLabel(e, log)
	default:
		return nil
	}
}

func (l *label) handleGenericCommentEvent(e *sdk.NoteEvent, log *logrus.Entry, action noteEventAction) error {
	body := *e.Note
	labelMatches := labelRegex.FindAllStringSubmatch(body, -1)
	removeLabelMatches := removeLabelRegex.FindAllStringSubmatch(body, -1)
	customLabelMatches := customLabelRegex.FindAllStringSubmatch(body, -1)
	customRemoveLabelMatches := customRemoveLabelRegex.FindAllStringSubmatch(body, -1)
	if len(labelMatches) == 0 && len(removeLabelMatches) == 0 && len(customLabelMatches) == 0 &&
		len(customRemoveLabelMatches) == 0 {
		return nil
	}
	repoLabels, err := l.ghc.GetRepoLabels(e.Repository.Namespace, e.Repository.Path)
	if err != nil {
		return err
	}
	labels, err := action.getAllLabels()
	if err != nil {
		return err
	}
	RepoLabelsExisting := sets.String{}
	for _, l := range repoLabels {
		RepoLabelsExisting.Insert(strings.ToLower(l.Name))
	}
	var (
		nonexistent         []string
		noSuchLabelsInRepo  []string
		noSuchLabelsOnIssue []string
		labelsToAdd         []string
		labelsToRemove      []string
	)
	var additionalLabels []string
	cfg, err := l.getLabelCfg()
	if err == nil {
		additionalLabels = append(additionalLabels, cfg.Label.AdditionalLabels...)
	}
	// Get labels to add and labels to remove from regexp matches
	labelsToAdd = append(getLabelsFromREMatches(labelMatches), getLabelsFromGenericMatches(customLabelMatches, additionalLabels, &nonexistent)...)
	labelsToRemove = append(getLabelsFromREMatches(removeLabelMatches), getLabelsFromGenericMatches(customRemoveLabelMatches, additionalLabels, &nonexistent)...)
	// Add labels
	for _, labelToAdd := range labelsToAdd {
		if plugins.HasLabel(labelToAdd, labels) {
			continue
		}

		if !RepoLabelsExisting.Has(labelToAdd) {
			noSuchLabelsInRepo = append(noSuchLabelsInRepo, labelToAdd)
			continue
		}
		if err := action.addLabel(labelToAdd); err != nil {
			log.WithError(err).Errorf("Gitee failed to add the following label: %s", labelToAdd)
		}
	}
	// Remove labels
	for _, labelToRemove := range labelsToRemove {
		if !plugins.HasLabel(labelToRemove, labels) {
			noSuchLabelsOnIssue = append(noSuchLabelsOnIssue, labelToRemove)
			continue
		}
		if !RepoLabelsExisting.Has(labelToRemove) {
			continue
		}
		if err := action.removeLabel(labelToRemove); err != nil {
			log.WithError(err).Errorf("Gitee failed to add the following label: %s", labelToRemove)
		}
	}
	if len(nonexistent) > 0 {
		log.Infof("Nonexistent labels: %v", nonexistent)
		msg := fmt.Sprintf("The label(s) `%s` cannot be applied. These labels are supported: `%s`",
			strings.Join(nonexistent, ", "), strings.Join(additionalLabels, ", "))

		return action.addComment(msg)
	}
	if len(noSuchLabelsInRepo) > 0 {
		log.Infof("Labels missing in repo: %v", noSuchLabelsInRepo)
		msg := fmt.Sprintf("The label(s) `%s` cannot be applied, because the repository doesn't have them",
			strings.Join(noSuchLabelsInRepo, ", "))

		return action.addComment(msg)
	}
	// Tried to remove Labels that were not present on the Issue
	if len(noSuchLabelsOnIssue) > 0 {
		msg := fmt.Sprintf("Those labels are not set: `%v`",
			strings.Join(noSuchLabelsOnIssue, ", "))

		return action.addComment(msg)
	}
	return nil
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

// getLabelsFromGenericMatches returns label matches with extra labels if those
// have been configured in the plugin config.
func getLabelsFromGenericMatches(matches [][]string, additionalLabels []string, invalidLabels *[]string) []string {
	if len(additionalLabels) == 0 {
		return nil
	}
	var labels []string
	labelFilter := sets.String{}
	for _, l := range additionalLabels {
		labelFilter.Insert(strings.ToLower(l))
	}
	for _, match := range matches {
		parts := strings.Split(strings.TrimSpace(match[0]), " ")
		if ((parts[0] != "/label") && (parts[0] != "/remove-label")) || len(parts) != 2 {
			continue
		}
		if labelFilter.Has(strings.ToLower(parts[1])) {
			labels = append(labels, parts[1])
		} else {
			*invalidLabels = append(*invalidLabels, match[0])
		}
	}
	return labels
}
