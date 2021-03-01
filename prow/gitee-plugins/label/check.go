package label

import (
	"fmt"
	"strings"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (l *label) handleCheckLimitLabel(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	cfg, err := l.getLabelCfg()
	if err != nil {
		return err
	}
	liLabel := cfg.Label.LimitLabels
	if len(liLabel) == 0 {
		return nil
	}
	needCheck := getLabelIntersection(e.PullRequest.Labels, liLabel)
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
	upLabel := getNeedUpdateLabels(e.PullRequest.Labels, clLabel)
	strLabel := strings.Join(upLabel, ",")
	org := e.Repository.Namespace
	repo := e.Repository.Path
	number := e.PullRequest.Number
	if _, err := l.ghc.UpdatePullRequest(org, repo, number, "", "", "", strLabel); err != nil {
		return err
	}
	comment := fmt.Sprintf(
		"These label(s): **%s** cannot be added by the author of the Pull request, so they have been removed.",
		strings.Join(clLabel, ","))
	return l.ghc.CreatePRComment(org, repo, int(number), comment)
}

func (l *label) handleClearLabel(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	cfg, err := l.getLabelCfg()
	if err != nil {
		return err
	}
	cll := cfg.Label.ClearLabels
	if len(cll) == 0 {
		log.Info("No labels to be cleared are configured when PR source branch has changed")
		return nil
	}
	needClear := getLabelIntersection(e.PullRequest.Labels, cll)
	if len(needClear) == 0 {
		return nil
	}
	needUpdate := getNeedUpdateLabels(e.PullRequest.Labels, needClear)
	strLabel := strings.Join(needUpdate, ",")
	org := e.Repository.Namespace
	repo := e.Repository.Path
	number := e.PullRequest.Number
	if _, err := l.ghc.UpdatePullRequest(org, repo, number, "", "", "", strLabel); err != nil {
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

func getLabelIntersection(labels []sdk.LabelHook, labels2 []string) []string {
	var iLabels []string
	labelSets := sets.String{}
	for _, l := range labels {
		labelSets.Insert(l.Name)
	}
	for _, v := range labels2 {
		if labelSets.Has(v) {
			iLabels = append(iLabels, v)
		}
	}
	return iLabels
}

func getNeedUpdateLabels(labels []sdk.LabelHook, cLabels []string) []string {
	labelSets := sets.String{}
	for _, l := range labels {
		labelSets.Insert(l.Name)
	}

	for _, v := range cLabels {
		if labelSets.Has(v) {
			labelSets.Delete(v)
		}
	}
	return labelSets.List()
}
