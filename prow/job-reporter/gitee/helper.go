package gitee

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/test-infra/prow/github"
)

var (
	jobsResultNotification   = "| Check Name | Result | Details |\n| --- | --- | --- |\n%s\n  <details>base sha:%s\nhead sha: %s</details>"
	jobsResultNotificationRe = regexp.MustCompile(fmt.Sprintf("\\| Check Name \\| Result \\| Details \\|\n\\| --- \\| --- \\| --- \\|\n%s\n  <details>base sha:%s\nhead sha: %s</details>", "([\\s\\S]*)", "(.*)", "(.*)"))
	jobStatusLabelRe         = regexp.MustCompile(`^ci/test-(error|failure|pending|success)$`)
)

type helper struct {
	comment *github.IssueComment
	jsc     *JobsComment
	labels  []string
	baseSHA string
	headSHA string
}

func newJobsComment() *JobsComment {
	return &JobsComment{
		JobsResultNotificationRe: jobsResultNotificationRe,
		JobComment:               jobComment{},
	}
}
func newHelper(c *ghclient, org, repo string, prNumber int) (*helper, error) {
	pr, err := c.GetGiteePullRequest(org, repo, prNumber)
	if err != nil {
		return nil, err
	}

	comments, err := c.ListIssueComments(org, repo, prNumber)
	if err != nil {
		return nil, err
	}

	jsc := newJobsComment()

	labels := make([]string, 0, len(pr.Labels))
	for i := range pr.Labels {
		labels = append(labels, pr.Labels[i].Name)
	}

	return &helper{
		comment: jsc.FindComment(c.botname, comments),
		jsc:     jsc,
		labels:  labels,
		baseSHA: pr.Base.Sha,
		headSHA: pr.Head.Sha,
	}, nil
}

// The three return values are: new comment, new job label, old job label that needs be deleted
func (h *helper) genCommentAndLabels(baseSHA, headSHA string, status *github.Status) (string, string, []string) {
	newComment := h.genNewComment(baseSHA, headSHA, status)
	if newComment == "" {
		return "", "", nil
	}

	currentLabels := sets.String{}
	for _, v := range h.labels {
		if jobStatusLabelRe.MatchString(v) {
			currentLabels.Insert(v)
		}
	}

	m := parseCommentElem(newComment)
	ss := h.jsc.ParseComment(m.jobResult)
	if len(ss) == 0 {
		return newComment, "", currentLabels.List()
	}

	l := genLabelByJobStatus(ss)
	if currentLabels.Has(l) {
		currentLabels.Delete(l)
		return newComment, "", currentLabels.List()
	}
	return newComment, l, currentLabels.List()
}

func (h *helper) genNewComment(baseSHA, headSHA string, status *github.Status) string {
	if h.comment == nil {
		s := ""
		if h.isSHAMatched(baseSHA, headSHA) {
			s = h.jsc.UpdateComment("", status)
		}
		return h.buildComment(s)
	}

	m := parseCommentElem(h.comment.Body)

	if h.isSHAMatched(m.baseSHA, m.headSHA) {
		if h.isSHAMatched(baseSHA, headSHA) {
			return h.buildComment(h.jsc.UpdateComment(m.jobResult, status))
		}
		return ""
	}

	s := ""
	if h.isSHAMatched(baseSHA, headSHA) {
		s = h.jsc.UpdateComment("", status)
	}
	return h.buildComment(s)
}

func (h *helper) isSHAMatched(baseSHA, headSHA string) bool {
	return h.baseSHA == baseSHA && h.headSHA == headSHA
}

func (h *helper) commentID() int {
	if h.comment == nil {
		return -1
	}
	return h.comment.ID
}

func (h *helper) buildComment(s string) string {
	return fmt.Sprintf(jobsResultNotification, s, h.baseSHA, h.headSHA)
}

func genLabelByJobStatus(ss []github.Status) string {
	statusSet := sets.String{}
	for _, item := range ss {
		statusSet.Insert(item.State)
	}

	if statusSet.Has(github.StatusError) {
		return "ci/test-error"
	}
	if statusSet.Has(github.StatusFailure) {
		return "ci/test-failure"
	}
	if statusSet.Has(github.StatusPending) {
		return "ci/test-pending"
	}
	return "ci/test-success"
}

type commentElem struct {
	jobResult string
	baseSHA   string
	headSHA   string
}

func parseCommentElem(s string) *commentElem {
	m := jobsResultNotificationRe.FindStringSubmatch(s)
	if m == nil {
		return nil
	}

	return &commentElem{
		jobResult: m[1],
		baseSHA:   m[2],
		headSHA:   m[3],
	}
}

func ParseCombinedStatus(botname, sha string, comments []github.IssueComment) []github.Status {
	jsc := newJobsComment()

	comment := jsc.FindComment(botname, comments)
	if comment == nil {
		return nil
	}

	m := parseCommentElem(comment.Body)
	if m.headSHA != sha {
		return nil
	}

	return jsc.ParseComment(m.jobResult)
}
