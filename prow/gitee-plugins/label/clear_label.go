package label

import (
	"fmt"
	"strings"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
)

func (l *label) handleClearLabel(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(e)
	if err != nil {
		return err
	}
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

	number := e.PullRequest.Number
	if err = l.removeLabels(org, repo, int(number), needClear, log); err != nil {
		return err
	}

	comment := fmt.Sprintf(
		"This pull request source branch has changed,label(s): **%s** has been removed.", strings.Join(needClear, ","))
	return l.ghc.CreatePRComment(org, repo, int(number), comment)
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
