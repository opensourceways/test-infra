package label

import (
	"fmt"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"strings"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (l *label) handleCheckLimitLabel(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(e)
	if err != nil {
		return err
	}
	number := e.PullRequest.Number
	cfg, err := l.orgRepoCfg(org, repo)
	if err != nil {
		return err
	}
	liLabel := cfg.LimitLabels
	if len(liLabel) == 0 {
		return nil
	}
	needCheck := getIntersectionOfLabels(e.PullRequest.Labels, liLabel)
	if len(needCheck) == 0 {
		return nil
	}
	clLabel, err := l.getAuthorAddLabels(e, needCheck)
	if err != nil {
		return err
	}
	if len(clLabel) == 0 {
		return nil
	}

	if err = l.removeLabels(org, repo, int(number), clLabel, log); err != nil {
		return err
	}

	comment := fmt.Sprintf(
		"These label(s): **%s** cannot be added by the author of the Pull request, so they have been removed.",
		strings.Join(clLabel, ","))
	return l.ghc.CreatePRComment(org, repo, int(number), comment)
}

func (l *label) handleClearLabel(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(e)
	if err != nil {
		return err
	}
	number := e.PullRequest.Number
	cfg, err := l.orgRepoCfg(org, repo)
	if err != nil {
		return err
	}
	cll := cfg.ClearLabels
	if len(cll) == 0 {
		log.Debug("No labels to be cleared are configured when PR source branch has changed")
		return nil
	}
	needClear := getIntersectionOfLabels(e.PullRequest.Labels, cll)
	if len(needClear) == 0 {
		return nil
	}
	if err = l.removeLabels(org, repo, int(number), needClear, log); err != nil {
		return err
	}

	comment := fmt.Sprintf("This pull request source branch has changed,label(s): **%s** has been removed.",
		strings.Join(needClear, ","))
	return l.ghc.CreatePRComment(org, repo, int(number), comment)
}

//getAuthorAddLabels get the restricted labels added by the PR author from the PR's operation log
func (l *label) getAuthorAddLabels(e *sdk.PullRequestEvent, checkLabels []string) ([]string, error) {
	var clearLabels []string
	logs, err := l.ghc.GetPullRequestOperateLogs(e.Repository.Namespace, e.Repository.Path, e.PullRequest.Number)
	if err != nil {
		return nil, err
	}
	for _, lb := range checkLabels {
		cc := fmt.Sprintf("添加了标签 %s", lb)
		for _, lg := range logs {
			if lg.Icon != "tag icon" {
				continue
			}
			if lg.Content == cc && lg.User.Login == e.PullRequest.User.Login {
				clearLabels = append(clearLabels, lb)
				break
			}
		}

	}
	return clearLabels, nil
}

func (l *label) removeLabels(org, repo string, number int, rms []string, log *logrus.Entry) error {
	ar := make([]string, 0, len(rms))
	for _, v := range rms {
		if err := l.ghc.RemovePRLabel(org, repo, number, v); err != nil {
			ar = append(ar, v)
		}
	}
	if len(ar) != 0 {
		return fmt.Errorf("remove %s label(s) occur error", strings.Join(ar, ","))
	}
	return nil
}

func getIntersectionOfLabels(labels []sdk.LabelHook, labels2 []string) []string {
	s1 := sets.String{}
	for _, l := range labels {
		s1.Insert(l.Name)
	}
	s2 := sets.NewString(labels2...)
	return s1.Intersection(s2).List()
}
