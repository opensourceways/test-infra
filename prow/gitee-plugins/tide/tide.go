package tide

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	prowConfig "k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pluginhelp"
)

var (
	checkPRRe          = regexp.MustCompile(`(?mi)^/check-pr\s*$`)
	tideNotification   = "@%s, This pr is **not** mergeable."
	tideNotificationRe = regexp.MustCompile(fmt.Sprintf(tideNotification, "(.*)"))
)

type giteeClient interface {
	CreatePRComment(owner, repo string, number int, comment string) error
	DeletePRComment(org, repo string, ID int) error
	ListPRComments(org, repo string, number int) ([]sdk.PullRequestComments, error)
	MergePR(owner, repo string, number int, opt sdk.PullRequestMergePutParam) error
}

type tide struct {
	getPluginConfig plugins.GetPluginConfig
	botName         string
	ghc             giteeClient
}

func NewPlugin(f plugins.GetPluginConfig, gec giteeClient, botName string) plugins.Plugin {
	return &tide{
		getPluginConfig: f,
		ghc:             gec,
		botName:         botName,
	}
}

func (t *tide) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The tide plugin tries to check the status of pr and merge it if possible.",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/check-pr",
		Description: "Forces rechecking the status of pr and merge it if possible.",
		Featured:    true,
		WhoCanUse:   "Anyone",
		Examples:    []string{"/check-pr"},
	})
	return pluginHelp, nil
}

func (t *tide) PluginName() string {
	return "tide"
}

func (t *tide) NewPluginConfig() plugins.PluginConfig {
	return &configuration{}
}

func (t *tide) RegisterEventHandler(p plugins.Plugins) {
	name := t.PluginName()
	p.RegisterNoteEventHandler(name, t.handleNoteEvent)
	p.RegisterPullRequestHandler(name, t.handlePullRequestEvent)
}

func (t *tide) handleNoteEvent(e *sdk.NoteEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handleNoteEvent")
	}()

	ne := gitee.NewPRNoteEvent(e)
	if !ne.IsCreatingCommentEvent() {
		log.Debug("Event is not a creation of a comment, skipping.")
		return nil
	}

	if !ne.IsPullRequest() {
		return nil
	}

	if !checkPRRe.MatchString(ne.GetComment()) {
		return nil
	}

	org, repo := ne.GetOrgRep()
	return t.handle(org, repo, e.PullRequest, log)
}

func (t *tide) handlePullRequestEvent(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handlePullRequest")
	}()

	if e.PullRequest.State != "open" {
		log.Debug("Pull request state is not open, skipping...")
		return nil
	}

	if plugins.ConvertPullRequestAction(e) != github.PullRequestActionLabeled {
		return nil
	}

	org, repo := gitee.GetOwnerAndRepoByPREvent(e)
	return t.handle(org, repo, e.PullRequest, log)
}

func (t *tide) handle(org, repo string, pr *sdk.PullRequestHook, log *logrus.Entry) error {
	cfg, err := t.orgRepoConfig(org, repo)
	if err != nil {
		return err
	}

	prNumber := int(pr.Number)
	author := pr.User.Login

	t.deleteOldComments(org, repo, prNumber)

	if !pr.Mergeable {
		return t.ghc.CreatePRComment(
			org, repo, prNumber,
			fmt.Sprintf(
				tideNotification+" Because it conflicts to the target branch.", author,
			),
		)
	}

	canMerge, desc := t.checkPrLabel(gitee.GetLabelFromEvent(pr.Labels), cfg, author)
	if canMerge {
		return t.ghc.MergePR(org, repo, prNumber, sdk.PullRequestMergePutParam{
			MergeMethod: string(cfg.MergeMethod),
		})
	}
	return t.ghc.CreatePRComment(org, repo, prNumber, desc)
}

func (t tide) deleteOldComments(org, repo string, prNumber int) error {
	comments, err := t.ghc.ListPRComments(org, repo, prNumber)
	if err != nil {
		return err
	}

	for _, c := range plugins.FindBotComment(comments, t.botName, tideNotificationRe) {
		t.ghc.DeletePRComment(org, repo, c.CommentID)
	}

	return nil
}

func (t tide) checkPrLabel(labels map[string]bool, cfg *pluginConfig, prAuthor string) (bool, string) {
	missing, exclude := cfg.labelDiff(labels)
	nm := len(missing)
	ne := len(exclude)
	if nm == 0 && ne == 0 {
		return true, ""
	}

	if nm != 0 && ne != 0 {
		return false, fmt.Sprintf(
			tideNotification+" It needs **%s** labels and needs to remove **%s** labels to get be merged.",
			prAuthor,
			strings.Join(missing, ", "),
			strings.Join(exclude, ", "),
		)
	}

	if nm != 0 {
		return false, fmt.Sprintf(
			tideNotification+" It needs **%s** labels to get be merged.",
			prAuthor,
			strings.Join(missing, ", "),
		)
	}

	return false, fmt.Sprintf(
		tideNotification+" It needs to remove **%s** labels to get be merged.",
		prAuthor,
		strings.Join(exclude, ", "),
	)
}

func (t *tide) orgRepoConfig(org, repo string) (*pluginConfig, error) {
	cfg, err := t.pluginConfig()
	if err != nil {
		return nil, err
	}

	pc := cfg.TideFor(org, repo)
	if pc == nil {
		return nil, fmt.Errorf("no cla plugin config for this repo:%s/%s", org, repo)
	}

	return pc, nil
}

func (t *tide) pluginConfig() (*configuration, error) {
	c := t.getPluginConfig(t.PluginName())
	if c == nil {
		return nil, fmt.Errorf("can't find the configuration")
	}

	c1, ok := c.(*configuration)
	if !ok {
		return nil, fmt.Errorf("can't convert to configuration")
	}

	return c1, nil
}
