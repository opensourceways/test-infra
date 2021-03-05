package cla

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
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

func (cl *cla) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
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

func (cl *cla) PluginName() string {
	return "cla"
}

func (cl *cla) NewPluginConfig() plugins.PluginConfig {
	return &configuration{}
}

func (cl *cla) RegisterEventHandler(p plugins.Plugins) {
	name := cl.PluginName()
	p.RegisterNoteEventHandler(name, cl.handleNoteEvent)
	p.RegisterPullRequestHandler(name, cl.handlePullRequestEvent)
}

func (cl *cla) handleNoteEvent(e *sdk.NoteEvent, log *logrus.Entry) error {
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
	repo := e.Repository.Path

	cfg, err := cl.orgRepoConfig(org, repo)
	if err != nil {
		return err
	}

	prNumber := int(pr.Number)
	cInf, signed, err := cl.getPrCommitsAbout(org, repo, prNumber, cfg.CheckURL)
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

	if signed {
		if hasCLANo {
			if err := cl.ghc.RemoveLabel(org, repo, prNumber, cfg.CLALabelNo); err != nil {
				log.WithError(err).Warningf("Could not remove %s label.", cfg.CLALabelNo)
			}
		}

		if !hasCLAYes {
			if err := cl.ghc.AddLabel(org, repo, prNumber, cfg.CLALabelYes); err != nil {
				log.WithError(err).Warningf("Could not add %s label.", cfg.CLALabelYes)
			}
		}
		return cl.ghc.CreateComment(org, repo, prNumber, alreadySigned(pr.Head.User.Login))
	}
	if hasCLAYes {
		if err := cl.ghc.RemoveLabel(org, repo, prNumber, cfg.CLALabelYes); err != nil {
			log.WithError(err).Warningf("Could not remove %s label.", cfg.CLALabelYes)
		}
	}

	if !hasCLANo {
		if err := cl.ghc.AddLabel(org, repo, prNumber, cfg.CLALabelNo); err != nil {
			log.WithError(err).Warningf("Could not add %s label.", cfg.CLALabelNo)
		}
	}
	return cl.ghc.CreateComment(org, repo, prNumber, signGuide(cfg.SignURL, "gitee", cInf))

}

func (cl *cla) handlePullRequestEvent(e *sdk.PullRequestEvent, log *logrus.Entry) error {
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
	repo := pr.Base.Repo.Path

	cfg, err := cl.orgRepoConfig(org, repo)
	if err != nil {
		return err
	}

	prNumber := int(pr.Number)
	cInf, signed, err := cl.getPrCommitsAbout(org, repo, prNumber, cfg.CheckURL)
	if err != nil {
		return err
	}
	if signed {
		if err := cl.ghc.AddLabel(org, repo, prNumber, cfg.CLALabelYes); err != nil {
			log.WithError(err).Warningf("Could not add %s label.", cfg.CLALabelYes)
		}
		return nil
	}

	if err := cl.ghc.AddLabel(org, repo, prNumber, cfg.CLALabelNo); err != nil {
		log.WithError(err).Warningf("Could not add %s label.", cfg.CLALabelNo)
	}
	return cl.ghc.CreateComment(org, repo, prNumber, signGuide(cfg.SignURL, "gitee", cInf))
}

func (cl *cla) orgRepoConfig(org, repo string) (*pluginConfig, error) {
	cfg, err := cl.pluginConfig()
	if err != nil {
		return nil, err
	}

	pc := cfg.CLAFor(org, repo)
	if pc == nil {
		return nil, fmt.Errorf("no cla plugin config for this repo:%s/%s", org, repo)
	}

	return pc, nil
}

func (cl *cla) pluginConfig() (*configuration, error) {
	c := cl.getPluginConfig(cl.PluginName())
	if c == nil {
		return nil, fmt.Errorf("can't find the configuration")
	}

	c1, ok := c.(*configuration)
	if !ok {
		return nil, fmt.Errorf("can't convert to configuration")
	}

	return c1, nil
}

func (cl *cla) getPrCommitsAbout(org, repo string, number int, checkURL string) (string, bool, error) {
	cos := map[string]*sdk.PullRequestCommits{}
	commits, err := cl.ghc.GetCommits(org, repo, number)
	if err != nil {
		return "", false, err
	}
	for i := range commits {
		v := &commits[i]
		if v.Commit == nil || v.Commit.Author == nil {
			continue
		}
		email := v.Commit.Author.Email
		if _, ok := cos[email]; !ok {
			cos[email] = v
		}
	}
	unSigns, signed, err := checkCommitsSigned(cos, checkURL)
	if err != nil {
		return "", signed, err
	}
	comment := generateUnSignComment(unSigns, cos)
	return comment, signed, err
}

func checkCommitsSigned(commits map[string]*sdk.PullRequestCommits, checkURL string) ([]string, bool, error) {
	if len(commits) == 0 {
		return nil, false, fmt.Errorf("commits is empty, cla cannot be checked")
	}
	var unCheck []string
	for k := range commits {
		signed, err := isSigned(k, checkURL)
		if err != nil {
			return unCheck, false, err
		}
		if !signed {
			unCheck = append(unCheck, k)
		}
	}
	return unCheck, len(unCheck) == 0, nil
}

func isSigned(email, url string) (bool, error) {
	endpoint := fmt.Sprintf("%s?email=%s", url, email)

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

func generateUnSignComment(unSigns []string, commits map[string]*sdk.PullRequestCommits) string {
	if len(unSigns) == 1 {
		return fmt.Sprintf("The author(**%s**) need to sign cla.", commits[unSigns[0]].Commit.Author.Name)
	}
	cs := make([]string, 0, len(unSigns))
	for _, v := range unSigns {
		tpl := "The author(**%s**) of commit [%s](%s) need to sign cla."
		com := commits[v]
		cs = append(cs, fmt.Sprintf(tpl, com.Commit.Author.Name, com.Sha[:8], com.HtmlUrl))
	}
	return strings.Join(cs, "\n")

}

func signGuide(signURL, platform, cInfo string) string {
	s := `Thanks for your pull request. Before we can look at your pull request, you'll need to sign a Contributor License Agreement (CLA).

%s
:memo: **Please access [here](%s) to sign the CLA.**

It may take a couple minutes for the CLA signature to be fully registered; after that, please reply here with a new comment: **/check-cla** to verify. Thanks.

---

- If you've already signed a CLA, it's possible you're using a different email address for your %s account. Check your existing CLA data and verify the email. 
- If you signed the CLA as an employee or a member of an organization, please contact your corporation or organization to verify you have been activated to start contributing.
- If you have done the above and are still having issues with the CLA being reported as unsigned, please feel free to file an issue.
	`

	return fmt.Sprintf(s, cInfo, signURL, platform)
}

func alreadySigned(user string) string {
	s := `***@%s***, thanks for your pull request. You've already signed CLA successfully. :wave: `
	return fmt.Sprintf(s, user)
}
