package reviewtrigger

import (
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/test-infra/prow/gitee-plugins/review-trigger/approvers"
)

type approverHelper struct {
	currentApprovers  []string
	assignees         []string
	filenames         []string
	prNumber          int
	numberOfApprovers int
	repoOwner         approvers.Repo
	prAuthor          string
	allowSelfApprove  bool
	log               *logrus.Entry
}

func (ah approverHelper) suggestApprovers() []string {
	currentApprovers1 := sets.NewString(ah.currentApprovers...)
	assignees1 := sets.NewString(ah.assignees...)
	prAuthor := ah.prAuthor
	dontAllowSelfApprove := !ah.allowSelfApprove

	if dontAllowSelfApprove {
		if currentApprovers1.Has(prAuthor) {
			currentApprovers1.Delete(prAuthor)
		}
		if assignees1.Has(prAuthor) {
			assignees1.Delete(prAuthor)
		}
	}

	owner := approvers.NewOwners(ah.log, ah.filenames, ah.repoOwner, int64(ah.prNumber))

	if ah.numberOfApprovers == 1 {
		ap := approvers.NewApprovers(owner)
		ap.AddAssignee(assignees1.UnsortedList()...)
		ap.AddApprover(currentApprovers1.UnsortedList()...)
		return ap.GetCCs()
	}

	approversAndAssignees := currentApprovers1.Union(assignees1)
	randomizedApprovers := owner.GetShuffledApprovers()
	leafReverseMap := approvers.GetReverseMap(owner.GetLeafApprovers())
	if dontAllowSelfApprove {
		if _, ok := leafReverseMap[prAuthor]; ok {
			delete(leafReverseMap, prAuthor)
			randomizedApprovers = removeSliceElement(randomizedApprovers, prAuthor)
		}
	}
	suggested := ah.keepCoveringApprovers(
		owner, leafReverseMap, approversAndAssignees, randomizedApprovers,
	)

	approversAndSuggested := currentApprovers1.Union(suggested)
	everyone := approversAndSuggested.Union(assignees1)
	fullReverseMap := approvers.GetReverseMap(owner.GetApprovers())
	if dontAllowSelfApprove {
		if _, ok := fullReverseMap[prAuthor]; ok {
			delete(fullReverseMap, prAuthor)
		}
	}
	keepAssignees := ah.keepCoveringApprovers(
		owner, fullReverseMap, approversAndSuggested, everyone.UnsortedList(),
	)

	return suggested.Union(keepAssignees).List()
}

func removeSliceElement(v []string, target string) []string {
	for i, item := range v {
		if item == target {
			n := len(v) - 1
			v[i] = v[n]
			return v[:n]
		}
	}
	return v
}

func (ah approverHelper) keepCoveringApprovers(owner approvers.Owners, reverseMap map[string]sets.String, knownApprovers sets.String, potentialApprovers []string) sets.String {
	numberOfApprovers := ah.numberOfApprovers

	f := func(ap *approvers.StaticApprovers) sets.String {
		excludedApprovers := sets.String{}
		unapproved := sets.String{}
		files := ap.GetFilesApprovers()
		for f, v := range files {
			if len(v) < numberOfApprovers {
				unapproved.Insert(f)
				for k := range v {
					excludedApprovers.Insert(k)
				}
			}
		}
		if len(unapproved) == 0 {
			return sets.NewString()
		}

		candidates := []string{}
		for _, item := range potentialApprovers {
			if !excludedApprovers.Has(item) {
				candidates = append(candidates, item)
			}
		}

		keptApprovers := sets.NewString()
		for suggestedApprover := range owner.GetSuggestedApprovers(reverseMap, candidates) {
			if reverseMap[suggestedApprover].Intersection(unapproved).Len() != 0 {
				keptApprovers.Insert(suggestedApprover)
			}
		}

		return keptApprovers
	}

	ap := approvers.NewStaticApprovers(owner, knownApprovers.UnsortedList())

	r := sets.NewString()
	for i := 0; i < numberOfApprovers; i++ {
		v := f(ap)
		if len(v) == 0 {
			break
		}

		r = r.Union(v)
		ap.AddApprover(v.UnsortedList()...)
	}

	return r
}
