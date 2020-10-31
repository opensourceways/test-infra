package cla

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	prowConfig "k8s.io/test-infra/prow/config"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pluginhelp"
)

var (
	checkCLARe = regexp.MustCompile(`(?mi)^/check-cla\s*$`)
)

type cla struct {
	getPluginConfig plugins.GetPluginConfig
	ghc             *ghclient
}

func NewCLA(f plugins.GetPluginConfig, gec giteeClient) plugins.Plugin {
	return &cla{
		getPluginConfig: f,
		ghc:             &ghclient{giteeClient: gec},
	}
}

func (this *cla) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The cla plugin manages the application and removal of the cla labels on pull requests. It is also responsible for warning unauthorized PR authors that they need to sign the cla before their PR will be merged.",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/check-cla",
		Description: "Forces rechecking of the CLA status.",
		Featured:    true,
		WhoCanUse:   "Anyone",
		Examples:    []string{"/check-cla"},
	})
	return pluginHelp, nil
}

func (this *cla) PluginName() string {
	return "cla"
}

func (this *cla) NewPluginConfig() plugins.PluginConfig {
	return &configuration{}
}

func (this *cla) RegisterEventHandler(p plugins.Plugins) {
	name := this.PluginName()
	p.RegisterNoteEventHandler(name, this.handleNoteEvent)
	p.RegisterPullRequestHandler(name, this.handlePullRequestEvent)
}

func (this *cla) handleNoteEvent(e *sdk.NoteEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handleNoteEvent")
	}()

	if *(e.Action) != "comment" {
		log.Debug("Event is not a creation of a comment, skipping.")
		return nil
	}

	if *(e.NoteableType) != "PullRequest" {
		return nil
	}

	// Only consider "/check-cla" comments.
	if !checkCLARe.MatchString(e.Comment.Body) {
		return nil
	}

	pr := e.PullRequest
	org := e.Repository.Namespace
	repo := e.Repository.Name

	cfg, err := this.orgRepoConfig(org, repo)
	if err != nil {
		return err
	}

	signed, err := isSigned(org, repo, pr.Head.User.Email, cfg.CheckURL)
	if err != nil {
		return err
	}

	hasCLAYes := false
	hasCLANo := false
	for _, label := range pr.Labels {
		if !hasCLAYes && label.Name == cfg.CLALabelYes {
			hasCLAYes = true
		}
		if !hasCLANo && label.Name == cfg.CLALabelNo {
			hasCLANo = true
		}
	}

	prNumber := int(pr.Number)

	if signed {
		if hasCLANo {
			if err := this.ghc.RemoveLabel(org, repo, prNumber, cfg.CLALabelNo); err != nil {
				log.WithError(err).Warningf("Could not remove %s label.", cfg.CLALabelNo)
			}
		}

		if !hasCLAYes {
			if err := this.ghc.AddLabel(org, repo, prNumber, cfg.CLALabelYes); err != nil {
				log.WithError(err).Warningf("Could not add %s label.", cfg.CLALabelYes)
			}
		}
		this.ghc.CreateComment(org, repo, prNumber, alreadySigned(pr.Head.User.Login))
	} else {
		if hasCLAYes {
			if err := this.ghc.RemoveLabel(org, repo, prNumber, cfg.CLALabelYes); err != nil {
				log.WithError(err).Warningf("Could not remove %s label.", cfg.CLALabelYes)
			}
		}

		if !hasCLANo {
			if err := this.ghc.AddLabel(org, repo, prNumber, cfg.CLALabelNo); err != nil {
				log.WithError(err).Warningf("Could not add %s label.", cfg.CLALabelNo)
			}
		}
		this.ghc.CreateComment(org, repo, prNumber, signGuide(cfg.SignURL, "gitee"))
	}
	return nil
}

func (this *cla) handlePullRequestEvent(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handlePullRequest")
	}()

	if e.PullRequest.State != "open" {
		log.Debug("Pull request state is not open, skipping...")
		return nil
	}

	action := plugins.ConvertPullRequestAction(e)
	if action != github.PullRequestActionOpened {
		return nil
	}

	pr := e.PullRequest
	org := pr.Base.Repo.Namespace
	repo := pr.Base.Repo.Name

	cfg, err := this.orgRepoConfig(org, repo)
	if err != nil {
		return err
	}

	signed, err := isSigned(org, repo, pr.Head.User.Email, cfg.CheckURL)
	if err != nil {
		return err
	}

	prNumber := int(pr.Number)
	if signed {
		if err := this.ghc.AddLabel(org, repo, prNumber, cfg.CLALabelYes); err != nil {
			log.WithError(err).Warningf("Could not add %s label.", cfg.CLALabelYes)
		}
		return nil
	}

	if err := this.ghc.AddLabel(org, repo, prNumber, cfg.CLALabelNo); err != nil {
		log.WithError(err).Warningf("Could not add %s label.", cfg.CLALabelNo)
	}

	this.ghc.CreateComment(org, repo, prNumber, signGuide(cfg.SignURL, "gitee"))
	return nil
}

func (this *cla) orgRepoConfig(org, repo string) (*pluginConfig, error) {
	cfg, err := this.pluginConfig()
	if err != nil {
		return nil, err
	}

	pc := cfg.CLAFor(org, repo)
	if pc == nil {
		return nil, fmt.Errorf("no cla plugin config for this repo:%s/%s", org, repo)
	}

	return pc, nil
}

func (this *cla) pluginConfig() (*configuration, error) {
	c := this.getPluginConfig(this.PluginName())
	if c == nil {
		return nil, fmt.Errorf("can't find the configuration")
	}

	c1, ok := c.(*configuration)
	if !ok {
		return nil, fmt.Errorf("can't convert to configuration")
	}

	return c1, nil
}

func isSigned(org, repo, email, url string) (bool, error) {
	endpoint := fmt.Sprintf("%s/%s/%s?email=%s", url, org, repo, email)

	resp, err := http.Get(endpoint)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	rb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return false, fmt.Errorf("response has status %q and body %q", resp.Status, string(rb))
	}

	type signingInfo struct {
		Signed bool `json:"signed"`
	}
	var v struct {
		Data signingInfo `json:"data"`
	}

	if err := json.Unmarshal(rb, &v); err != nil {
		return false, fmt.Errorf("unmarshal failed: %s", err.Error())
	}
	return v.Data.Signed, nil
}

func signGuide(signURL, platform string) string {
	s := `Thanks for your pull request. Before we can look at your pull request, you'll need to sign a Contributor License Agreement (CLA).

:memo: **Please access [here](%s) to sign the CLA.**

It may take a couple minutes for the CLA signature to be fully registered; after that, please reply here with a new comment: **/check-cla** to verify. Thanks.

---

- If you've already signed a CLA, it's possible you're using a different email address for your %s account. Check your existing CLA data and verify the email. 
- If you signed the CLA as an employee or a member of an organization, please contact your corporation or organization to verify you have been activated to start contributing.
- If you have done the above and are still having issues with the CLA being reported as unsigned, please feel free to file an issue.
	`

	return fmt.Sprintf(s, signURL, platform)
}

func alreadySigned(user string) string {
	s := `***@%s***, thanks for your pull request. You've already signed CLA successfully. :wave: `
	return fmt.Sprintf(s, user)
}
