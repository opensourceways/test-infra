package reviewtrigger

import (
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/test-infra/prow/repoowners"
)

type reviewerHelper struct {
	org      string
	repo     string
	prAuthor string
	prNumber int

	c   ghclient
	roc repoowners.RepoOwner
	log *logrus.Entry
	cfg reviewerConfig
}

func (r reviewerHelper) suggestReviewers() ([]string, error) {
	changes, err := r.c.getPullRequestChanges(r.org, r.repo, r.prNumber)
	if err != nil {
		return nil, fmt.Errorf("error getting PR changes: %v", err)
	}

	reviewerCount := r.cfg.ReviewerCount
	excludedReviewers := sets.NewString(r.prAuthor)
	reviewers := r.getReviewers(r.roc, changes, reviewerCount, excludedReviewers)
	if len(reviewers) < reviewerCount && !r.cfg.ExcludeApprovers {
		approvers := r.getReviewers(
			fallbackReviewersClient{oc: r.roc},
			changes, reviewerCount-len(reviewers),
			excludedReviewers.Insert(reviewers...),
		)
		reviewers = append(reviewers, approvers...)
		r.log.Infof(
			"Added %d approvers as reviewers. %d/%d reviewers found.",
			len(approvers), len(reviewers), reviewerCount,
		)
	}

	if len(reviewers) < reviewerCount {
		r.log.Warnf(
			"Not enough reviewers found in OWNERS files for files touched by this PR. %d/%d reviewers found.",
			len(reviewers), reviewerCount,
		)
	}

	return reviewers, nil
}

func (r reviewerHelper) getReviewers(rc reviewersClient, files []string, minReviewers int, excludedReviewers sets.String) []string {
	leafReviewers := sets.NewString()
	for _, filename := range files {
		v := rc.LeafReviewers(filename).Difference(excludedReviewers)
		if v.Len() > 0 {
			leafReviewers = leafReviewers.Union(v)
		}
	}

	if leafReviewers.Len() >= minReviewers {
		reviewers := sets.NewString()
		for reviewers.Len() < minReviewers {
			if r := findReviewer(&leafReviewers); r != "" {
				reviewers.Insert(r)
			}
		}
		return reviewers.List()
	}

	reviewers := leafReviewers
	fileReviewers := sets.NewString()
	for _, filename := range files {
		v := rc.Reviewers(filename).Difference(excludedReviewers).Difference(reviewers)
		if v.Len() > 0 {
			fileReviewers = fileReviewers.Union(v)
		}
	}
	for reviewers.Len() < minReviewers && fileReviewers.Len() > 0 {
		if r := findReviewer(&fileReviewers); r != "" {
			reviewers.Insert(r)
		}
	}
	return reviewers.List()
}

// popRandom randomly selects an element of 'set' and pops it.
func popRandom(set *sets.String) string {
	list := set.List()
	sort.Strings(list)
	sel := list[rand.Intn(len(list))]
	set.Delete(sel)
	return sel
}

// findReviewer finds a reviewer from a set randomly.
func findReviewer(targetSet *sets.String) string {
	return popRandom(targetSet)
}

type reviewTips = approveTips

func findReviewTips(comments []sdk.PullRequestComments, botName string) reviewTips {
	reStr := strings.ReplaceAll(suggestComment, "( %s )", "\\( %s \\)")
	reviewTipRe := regexp.MustCompile(fmt.Sprintf(reStr, "(.*)", "(.*)", "(.*)"))
	for i := range comments {
		tip := &comments[i]
		if tip.User == nil || tip.User.Login != botName {
			continue
		}
		if reviewTipRe.MatchString(tip.Body) {
			return reviewTips{tipsID: int(tip.Id), body: tip.Body}
		}
	}
	return reviewTips{}
}

type reviewersClient interface {
	Reviewers(path string) sets.String
	LeafReviewers(path string) sets.String
}

type fallbackReviewersClient struct {
	oc repoowners.RepoOwner
}

func (foc fallbackReviewersClient) Reviewers(path string) sets.String {
	return foc.oc.Approvers(path)
}

func (foc fallbackReviewersClient) LeafReviewers(path string) sets.String {
	return foc.oc.LeafApprovers(path)
}
