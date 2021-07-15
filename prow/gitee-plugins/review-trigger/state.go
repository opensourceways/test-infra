package reviewtrigger

import (
	"fmt"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/test-infra/prow/repoowners"
)

type reviewState struct {
	org            string
	repo           string
	headSHA        string
	botName        string
	prAuthor       string
	prNumber       int
	filenames      []string
	currentLabels  map[string]bool
	assignees      []string
	c              ghclient
	cfg            *pluginConfig
	dirApproverMap map[string]sets.String
	approverDirMap map[string]sets.String
	reviewers      sets.String
	owner          repoowners.RepoOwner
	log            *logrus.Entry
}

func (rs reviewState) handle(isCIPassed bool, latestComment string) error {
	t, err := rs.c.getPRCodeUpdateTime(rs.org, rs.repo, rs.headSHA)
	if err != nil {
		return err
	}

	comments, err := rs.c.ListPRComments(rs.org, rs.repo, rs.prNumber)
	if err != nil {
		return err
	}

	validComments := rs.filterComments(comments, t)
	if len(validComments) == 0 {
		return nil
	}

	label := rs.applyComments(validComments)

	return rs.applyLabel(label, validComments, comments, isCIPassed, latestComment)
}

func (rs reviewState) applyLabel(label string, reviewComments []*sComment, allComments []sdk.PullRequestComments, isCIPassed bool, latestComment string) error {
	cls := rs.currentLabels
	tips := findApproveTips(allComments, rs.botName)

	switch label {
	case labelRequestChange:
		return rs.applyRequestChange(cls, reviewComments, tips)
	case labelLGTM:
		return rs.applyLGTM(cls, reviewComments, isCIPassed, latestComment, tips)
	case labelApproved:
		return rs.applyApproved(cls, reviewComments, tips)
	}

	return nil
}

func (rs reviewState) applyLGTM(cls map[string]bool, reviewComments []*sComment, isCIPassed bool, latestComment string, oldTips botComment) error {
	errs := newErrors()
	err := rs.applyLGTMLabel(cls)
	errs.addError(err)

	approvers, reviewers := statOnesWhoAgreed(reviewComments)
	var sa []string
	if rs.cfg.NoCI || isCIPassed || cls[rs.cfg.LabelForCIPassed] {
		if !oldTips.exists() || !doesTipsHasPart2(oldTips.body) || latestComment == cmdAPPROVE {
			sa = rs.suggestApprover(approvers)
		}
	}

	desc := lgtmTips(approvers, reviewers, sa, oldTips.body)
	err = rs.writeApproveTips(desc, oldTips)
	errs.addError(err)

	return errs.err()
}

func (rs reviewState) applyRequestChange(cls map[string]bool, reviewComments []*sComment, oldTips botComment) error {
	errs := newErrors()
	err := rs.applyRequestChangeLabel(cls)
	errs.addError(err)

	rejecters, reviewers := statOnesWhoDisagreed(reviewComments)

	desc := ""
	if len(rejecters) == 0 {
		desc = requestChangeTips(reviewers)
	} else {
		desc = rejectTips(rejecters)
	}
	err = rs.writeApproveTips(desc, oldTips)
	errs.addError(err)

	return errs.err()
}

func (rs reviewState) applyApproved(cls map[string]bool, reviewComments []*sComment, oldTips botComment) error {
	errs := newErrors()
	err := rs.applyApprovedLabel(cls)
	errs.addError(err)

	approvers, _ := statOnesWhoAgreed(reviewComments)
	err = rs.writeApproveTips(approvedTips(approvers), oldTips)
	errs.addError(err)

	return errs.err()
}

func (rs reviewState) writeApproveTips(desc string, oldTips botComment) error {
	// can't check `if desc == oldTips.body` instead
	if desc == "" || desc == oldTips.body {
		return nil
	}

	if oldTips.exists() {
		rs.c.DeletePRComment(rs.org, rs.repo, oldTips.commentID)
	}
	return rs.c.CreatePRComment(rs.org, rs.repo, rs.prNumber, desc)
}

func (rs reviewState) applyApprovedLabel(cls map[string]bool) error {
	toAdd := []string{}

	if !cls[labelApproved] {
		toAdd = append(toAdd, labelApproved)
	}

	if !cls[labelLGTM] {
		toAdd = append(toAdd, labelLGTM)
	}

	errs := newErrors()

	if len(toAdd) > 0 {
		err := rs.c.AddMultiPRLabel(rs.org, rs.repo, rs.prNumber, toAdd)
		errs.addError(err)
	}

	toRemove := []string{labelRequestChange, labelCanReview}
	for _, l := range toRemove {
		if cls[l] {
			err := rs.c.RemovePRLabel(rs.org, rs.repo, rs.prNumber, l)
			errs.addError(err)
		}
	}

	return errs.err()
}

func (rs reviewState) applyLGTMLabel(cls map[string]bool) error {
	errs := newErrors()

	l := labelLGTM
	if !cls[l] {
		if err := rs.c.AddPRLabel(rs.org, rs.repo, rs.prNumber, l); err != nil {
			errs.addError(err)
		} else {
			err := rs.c.CreatePRComment(
				rs.org, rs.repo, rs.prNumber, fmt.Sprintf("%s label has been added.", l),
			)
			errs.addError(err)
		}
	}

	for _, l := range []string{labelApproved, labelRequestChange, labelCanReview} {
		if cls[l] {
			err := rs.c.RemovePRLabel(rs.org, rs.repo, rs.prNumber, l)
			errs.addError(err)
		}
	}

	return errs.err()
}

func (rs reviewState) applyRequestChangeLabel(cls map[string]bool) error {
	errs := newErrors()

	l := labelRequestChange
	if !cls[l] {
		if err := rs.c.AddPRLabel(rs.org, rs.repo, rs.prNumber, l); err != nil {
			errs.addError(err)
		} else {
			err := rs.c.CreatePRComment(
				rs.org, rs.repo, rs.prNumber, fmt.Sprintf("%s label has been added.", l),
			)
			errs.addError(err)
		}
	}

	for _, l := range []string{labelApproved, labelLGTM, labelCanReview} {
		if cls[l] {
			err := rs.c.RemovePRLabel(rs.org, rs.repo, rs.prNumber, l)
			errs.addError(err)
		}
	}

	return errs.err()
}

func (rs reviewState) isApprover(author string) bool {
	_, b := rs.approverDirMap[author]
	return b
}

func (rs reviewState) dirsOfApprover(author string) sets.String {
	v, b := rs.approverDirMap[author]
	if b {
		return v
	}

	return sets.String{}
}

func (rs reviewState) isReviewer(author string) bool {
	return rs.reviewers.Has(author)
}

func (rs reviewState) isAllFilesApproved(as []string) bool {
	m := map[string]bool{}
	for _, a := range as {
		d, ok := rs.approverDirMap[a]
		if !ok {
			continue
		}

		for i := range d {
			if !m[i] {
				m[i] = true
			}
		}
	}
	return len(m) == len(rs.filenames)
}

func (rs reviewState) selectApprovers(as []string, n int) []string {
	excluded := map[string]bool{}
	for _, a := range as {
		excluded[a] = true
	}

	if !rs.cfg.AllowSelfApprove {
		excluded[rs.prAuthor] = true
	}

	r := make([]string, 0, n)
	done := func() bool {
		return len(r) >= n
	}

	find := func(getCandidates func(string) sets.String) {
		for _, f := range rs.filenames {
			for i := range getCandidates(f) {
				if !excluded[i] {
					r = append(r, i)
					if done() {
						break
					}
					excluded[i] = true
				}
			}
			if done() {
				break
			}
		}
	}

	find(rs.owner.LeafApprovers)
	if !done() {
		find(func(p string) sets.String {
			if v, ok := rs.dirApproverMap[p]; ok {
				return v
			}
			return sets.NewString()
		})
	}
	return r
}

func (rs reviewState) suggestApproverOfMiddleLevel(currentApprovers []string) []string {
	f := func(suggested []string) []string {
		n := rs.cfg.TotalNumberOfApprovers - len(currentApprovers) - len(suggested)
		if n <= 0 {
			return suggested
		}
		v := rs.selectApprovers(mergeSlices(currentApprovers, suggested), n)
		v = append(v, suggested...)
		return v
	}

	as := mergeSlices(currentApprovers, rs.assignees)
	if rs.isAllFilesApproved(as) {
		if len(rs.assignees) > 0 {
			suggested := make([]string, 0, len(rs.assignees))
			for _, i := range rs.assignees {
				if rs.isApprover(i) {
					suggested = append(suggested, i)
				}
			}
			return f(difference(suggested, currentApprovers))
		}
		return f([]string{})
	}

	return f(rs.suggestApproverByNumber(currentApprovers))
}

func (rs reviewState) suggestApprover(currentApprovers []string) []string {
	if rs.cfg.MiddleLevel {
		return rs.suggestApproverOfMiddleLevel(currentApprovers)
	}

	return rs.suggestApproverByNumber(currentApprovers)
}

func (rs reviewState) suggestApproverByNumber(currentApprovers []string) []string {
	ah := approverHelper{
		currentApprovers:  currentApprovers,
		assignees:         rs.assignees,
		filenames:         rs.filenames,
		prNumber:          rs.prNumber,
		numberOfApprovers: rs.cfg.NumberOfApprovers,
		repoOwner:         rs.owner,
		prAuthor:          rs.prAuthor,
		allowSelfApprove:  rs.cfg.AllowSelfApprove,
		log:               rs.log,
	}
	return ah.suggestApprovers()
}

func mergeSlices(s []string, s1 []string) []string {
	r := make([]string, 0, len(s)+len(s1))
	r = append(r, s...)
	r = append(r, s1...)
	return r
}

func difference(s, s1 []string) []string {
	if len(s) == 0 || len(s1) == 0 {
		return s
	}
	return sets.NewString(s...).Difference(
		sets.NewString(s1...),
	).List()
}
