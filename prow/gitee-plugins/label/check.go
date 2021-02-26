package label

import (
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

	return nil
}

func (l *label) handleClearLabel(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	cfg, err := l.getLabelCfg()
	if err != nil {
		return err
	}
	cll := cfg.Label.ClearLabels
	if len(cll) == 0 {
		return nil
	}
	needClear := getLabelIntersection(e.PullRequest.Labels, cll)
	if len(needClear) == 0 {
		return nil
	}
	needUpdate := getNeedUpdateLabels(e.PullRequest.Labels, needClear)
	return nil
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
	var updateLabels []string
	labelSets := sets.String{}
	for _, l := range labels {
		labelSets.Insert(l.Name)
	}
	for _, v := range cLabels {
		if !labelSets.Has(v) {
			updateLabels = append(updateLabels, v)
		}
	}
	return updateLabels
}
