package reminder

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	prowConfig "k8s.io/test-infra/prow/config"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/pluginhelp"
)

var (
	defaultLabels = []string{"kind", "priority", "area"}
	labelRegex    = regexp.MustCompile(`(?m)^//(comp|sig)\s*(.*?)\s*$`)
)

type reminder struct {
	getPluginConfig plugins.GetPluginConfig
	ghc             *ghclient
}

func NewReminder(f plugins.GetPluginConfig, gec giteeClient) plugins.Plugin {
	return &reminder{
		getPluginConfig: f,
		ghc:             &ghclient{giteeClient: gec},
	}
}

func (re *reminder) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "Labels are essential for issue responding, this tool can remind issue participants to offer the labels",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "reminder : aoto-trigger ; add labels : // <lable type> / <label subtype>",
		Description: "remind issue participants to add labels",
		Featured:    true,
		WhoCanUse:   "Anyone",
		Examples:    []string{"//comp/data"},
	})
	return pluginHelp, nil
}

func (re *reminder) PluginName() string {
	return "reminder"
}

func (re *reminder) NewPluginConfig() plugins.PluginConfig {
	return &configuration{}
}

func (re *reminder) RegisterEventHandler(p plugins.Plugins) {
	name := re.PluginName()
	p.RegisterIssueHandler(name, re.handleIssueEvent)
	p.RegisterNoteEventHandler(name, re.handleNoteEvent)
}

func (re *reminder) handleIssueEvent(e *sdk.IssueEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handleIssueEvent")
	}()

	if *(e.Action) != "open" {
		log.Debug("Event is not a creation of a issue, skipping.")
		return nil
	}
	HELPLabel := "stat/help-wanted"
	issue := e.Issue
	org := e.Repository.Namespace
	repo := e.Repository.Path
	labels := issue.Labels
	issueNumber := issue.Number

	hasHELP := false

	for _, label := range labels {
		if !hasHELP && label.Name == HELPLabel {
			hasHELP = true
			break
		}
	}

	if !hasHELP {
		if err := re.ghc.CreateComment(org, repo, issueNumber, "Please add labels, for example, "+
			`if you are filing a runtime issue, you can type "//comp/runtime" in comment,`+
			` also you can visit "https://shimo.im/sheets/8pKDkqKqdycHRwWV/MODOC/" to find more labels`); err != nil {
			log.WithError(err).Warningf("Could not add label.")
		}
	}
	return re.ghc.AddLabel(org, repo, issueNumber, HELPLabel)
}

func (re *reminder) handleNoteEvent(e *sdk.NoteEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handleNoteEvent")
	}()

	if *(e.Action) != "comment" {
		log.Debug("Event is not a creation of a comment, skipping.")
		return nil
	}

	userName := e.Comment.User.Name
	org := e.Repository.Namespace
	repo := e.Repository.Path
	issueNumber := e.Issue.Number
	noteBody := e.Comment.Body
	noteType := e.NoteableType
	botName, err := re.ghc.getBotName()
	if err != nil {
		return err
	}

	labelMatches := labelRegex.FindAllStringSubmatch(noteBody, -1)
	if len(labelMatches) == 0 {
		return nil
	}

	var labelsToAdd []string
	// Get labels to add and labels to remove from regexp matches
	labelsToAdd = getLabelsFromREMatches(labelMatches)

	// Add labels
	if userName != botName && *noteType == "Issue" {
		return re.ghc.AddLabels(org, repo, issueNumber, labelsToAdd)
	}
	return nil
}

func (re *reminder) orgRepoConfig(org, repo string) (*pluginConfig, error) {
	cfg, err := re.pluginConfig()
	if err != nil {
		return nil, err
	}

	pc := cfg.ReminderFor(org, repo)
	if pc == nil {
		return nil, fmt.Errorf("no reminder plugin config for this repo:%s/%s", org, repo)
	}
	return pc, nil
}

func (re *reminder) pluginConfig() (*configuration, error) {
	c := re.getPluginConfig(re.PluginName())
	if c == nil {
		return nil, fmt.Errorf("can't find the configuration")
	}

	c1, ok := c.(*configuration)
	if !ok {
		return nil, fmt.Errorf("can't convert to configuration")
	}

	return c1, nil
}

func getLabelsFromREMatches(matches [][]string) (labels []string) {
	for _, match := range matches {
		label := strings.TrimSpace(strings.Trim(match[0],"//"))
		labels = append(labels, label)
	}
	return
}
