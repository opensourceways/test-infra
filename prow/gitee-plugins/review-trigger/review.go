package reviewtrigger

import (
	"fmt"
	"regexp"
	"strings"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	prowConfig "k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pluginhelp"
	"k8s.io/test-infra/prow/repoowners"
)

const (
	labelCanReview     = "can-review"
	labelLGTM          = "lgtm"
	labelApproved      = "approved"
	labelRequestChange = "request-change"
)

var (
	prefixOfSuggestingReviewer   = "@%s, suggests these reviewers( %s ) to review your code."
	prefixOfSuggestingReviewerRe = regexp.MustCompile("@(.*), suggests these reviewers\\( (.*) \\) to review your code")
)

type trigger struct {
	client          ghclient
	botName         string
	oc              repoowners.Interface
	getPluginConfig plugins.GetPluginConfig
}

func NewPlugin(f plugins.GetPluginConfig, gc giteeClient, botName string, oc repoowners.Interface) plugins.Plugin {
	return &trigger{
		getPluginConfig: f,
		oc:              oc,
		botName:         botName,
		client:          ghclient{giteeClient: gc},
	}
}

func (rt *trigger) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: `The review_trigger plugin will trigger the whole review process to merge pull-request.
		It will handle comment of reviewer and approver, such as /lgtm, /lbtm, /approve and /reject.
		Also, it can add label of CI test cases.
		`,
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/lgtm",
		Description: "The code looks good. It will add 'lgtm' label if reviewer comment /lgtm",
		Featured:    true,
		WhoCanUse:   "Anyone except the author of pull-request",
		Examples:    []string{"/lgtm"},
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/lbtm",
		Description: "The code looks bad. It will add 'request-change' label if reviewer comment /lbtm",
		Featured:    true,
		WhoCanUse:   "Anyone",
		Examples:    []string{"/lbtm"},
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/approve",
		Description: "The code is ready to be merged. It may add 'approved' label if approver comment /approve",
		Featured:    true,
		WhoCanUse:   "The approver",
		Examples:    []string{"/approve"},
	})
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/reject",
		Description: "The code can't be merged. It will add 'request-change' label if approver comment /reject",
		Featured:    true,
		WhoCanUse:   "The approvers except the author of pull-request",
		Examples:    []string{"/reject"},
	})

	return pluginHelp, nil
}

func (rt *trigger) PluginName() string {
	return "review_trigger"
}

func (rt *trigger) NewPluginConfig() plugins.PluginConfig {
	return &configuration{}
}

func (rt *trigger) orgRepoConfig(org, repo string) (*pluginConfig, error) {
	cfg, err := rt.pluginConfig()
	if err != nil {
		return nil, err
	}

	pc := cfg.TriggerFor(org, repo)
	if pc == nil {
		return nil, fmt.Errorf("no cla plugin config for this repo:%s/%s", org, repo)
	}

	return pc, nil
}

func (rt *trigger) pluginConfig() (*configuration, error) {
	c := rt.getPluginConfig(rt.PluginName())
	if c == nil {
		return nil, fmt.Errorf("can't find the configuration")
	}

	c1, ok := c.(*configuration)
	if !ok {
		return nil, fmt.Errorf("can't convert to configuration")
	}

	return c1, nil
}
func (rt *trigger) RegisterEventHandler(p plugins.Plugins) {
	name := rt.PluginName()
	p.RegisterNoteEventHandler(name, rt.handleNoteEvent)
	p.RegisterPullRequestHandler(name, rt.handlePREvent)
}

func (rt *trigger) handlePREvent(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	action := plugins.ConvertPullRequestAction(e)
	org, repo := gitee.GetOwnerAndRepoByPREvent(e)
	prNumber := int(e.PullRequest.Number)
	errs := newErrors()

	switch action {
	case github.PullRequestActionOpened:
		if err := rt.client.AddPRLabel(org, repo, prNumber, labelCanReview); err != nil {
			errs.add(fmt.Sprintf("add label when pr is open, err:%s", err.Error()))
		}

		if err := rt.welcome(org, repo, prNumber); err != nil {
			errs.add(fmt.Sprintf("add welcome comment, err:%s", err.Error()))
		}

		if err := rt.suggestReviewers(e, log); err != nil {
			errs.add(fmt.Sprintf("suggest reviewers, err: %s", err.Error()))
		}

		// no need to update local repo everytime when a pr is open.
		// repoowner will update it necessarily when suggesting reviewers.

	case github.PullRequestActionSynchronize:
		if err := rt.removeInvalidLabels(e, true); err != nil {
			errs.add(fmt.Sprintf("remove label when source code changed, err:%s", err.Error()))
		}

		if err := rt.deleteOldComments(org, repo, prNumber); err != nil {
			errs.add(fmt.Sprintf("delete tips, err:%s", err.Error()))
		}

		if err := rt.suggestReviewers(e, log); err != nil {
			errs.add(fmt.Sprintf("suggest reviewers, err: %s", err.Error()))
		}
	}
	return errs.err()
}

func (rt *trigger) welcome(org, repo string, prNumber int) error {
	cfg, err := rt.pluginConfig()
	if err != nil {
		return err
	}

	return rt.client.CreatePRComment(
		org, repo, prNumber,
		fmt.Sprintf(
			"Thank your for your pull-request.\n\nThe full list of commands accepted by me can be found at [**here**](%s).\nYou can get details about the review process of pull-request at [**here**](%s)",
			cfg.Trigger.commandsLink(org, repo), "https://github.com/opensourceways/test-infra/blob/sync-5-22/prow/gitee-plugins/review-trigger/review.md",
		),
	)
}

func (rt *trigger) removeInvalidLabels(e *sdk.PullRequestEvent, canReview bool) error {
	org, repo := gitee.GetOwnerAndRepoByPREvent(e)
	cfg, err := rt.orgRepoConfig(org, repo)
	if err != nil {
		return err
	}

	rml := []string{labelApproved, labelRequestChange, labelLGTM}
	if cfg.EnableLabelForCI {
		rml = append(rml, cfg.labelsForCI()...)
	}
	if !canReview {
		rml = append(rml, labelCanReview)
	}

	number := int(e.PullRequest.Number)
	m := gitee.GetLabelFromEvent(e.PullRequest.Labels)

	rmls := make([]string, 0, len(rml))
	errs := newErrors()
	for _, l := range rml {
		if m[l] {
			if err := rt.client.RemovePRLabel(org, repo, number, l); err != nil {
				errs.add(fmt.Sprintf("remove label:%s, err:%v", l, err))
			}
			rmls = append(rmls, l)
		}
	}
	if len(rmls) > 0 {
		rt.client.CreatePRComment(
			org, repo, number, fmt.Sprintf(
				"New changes are detected. Remove the following labels: %s.",
				strings.Join(rmls, ", "),
			),
		)
	}

	l := labelCanReview
	if canReview && !m[l] {
		if err := rt.client.AddPRLabel(org, repo, number, l); err != nil {
			errs.add(fmt.Sprintf("add label:%s, err:%v", l, err))
		}
	}

	return errs.err()
}

func (rt *trigger) deleteOldComments(org, repo string, prNumber int) error {
	comments, err := rt.client.ListPRComments(org, repo, prNumber)
	if err != nil {
		return err
	}

	if tips := findApproveTips(comments, rt.botName); tips.exists() {
		rt.client.DeletePRComment(org, repo, tips.tipsID)
	}

	if c := findBotComment(comments, rt.botName, prefixOfSuggestingReviewerRe); c.exists() {
		rt.client.DeletePRComment(org, repo, c.commentID)
	}

	return nil
}

func (rt *trigger) handleNoteEvent(e *sdk.NoteEvent, log *logrus.Entry) error {
	ne := gitee.NewPRNoteEvent(e)
	if !ne.IsPullRequest() || !ne.IsPROpen() {
		return nil
	}

	if ne.IsCreatingCommentEvent() && ne.GetCommenter() != rt.botName {
		cmds := parseCommandFromComment(ne.GetComment())
		if len(cmds) > 0 {
			return rt.handleReviewComment(ne, log)
		}
	}

	return rt.handleCIStatusComment(ne, log)
}

func (rt *trigger) suggestReviewers(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	org, repo := gitee.GetOwnerAndRepoByPREvent(e)
	cfg, err := rt.orgRepoConfig(org, repo)
	if err != nil {
		return err
	}

	pr := e.PullRequest

	ow, err := rt.newRepoOwner(org, repo, pr.Base.Ref, cfg, log)
	if err != nil {
		return err
	}

	sg := reviewerHelper{
		org:      org,
		repo:     repo,
		prAuthor: github.NormLogin(pr.User.Login),
		prNumber: int(pr.Number),
		c:        rt.client,
		roc:      ow,
		log:      log,
		cfg:      cfg.Reviewers,
	}
	reviewers, err := sg.suggestReviewers()
	if err != nil {
		return err
	}
	if len(reviewers) == 0 {
		return nil
	}

	rs := convertReviewers(reviewers)
	return rt.client.CreatePRComment(
		org, repo, sg.prNumber, fmt.Sprintf(
			prefixOfSuggestingReviewer+" You can ask one of them by writing `@%s` in a comment",
			sg.prAuthor, strings.Join(rs, ", "), reviewers[0],
		),
	)
}
