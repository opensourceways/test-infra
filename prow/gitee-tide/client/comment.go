package client

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/test-infra/prow/github"
	reporter "k8s.io/test-infra/prow/job-reporter/gitee"
)

type jobStatusComment struct {
	JobsResultNotification   string
	JobsResultNotificationRe *regexp.Regexp
	JobResultNotification    string
	JobResultNotificationRe  *regexp.Regexp
}

func (j *jobStatusComment) findCheckResultComment(botname string, comments []github.IssueComment) (string, string, int) {
	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]
		if comment.User.Login != botname {
			continue
		}

		m := j.JobsResultNotificationRe.FindStringSubmatch(comment.Body)
		if m != nil {
			return m[1], m[2], comment.ID
		}
	}

	return "", "", -1
}

func (j *jobStatusComment) buildJobResultComment(s github.Status) string {
	icon := reporter.StateToIcon(s.State)
	return fmt.Sprintf(j.JobResultNotification, icon, s.Context, s.Description, s.TargetURL)
}

func (j *jobStatusComment) genJobResultComment(jobsOldComment, oldSha, newSha string, jobStatus github.Status) string {
	jobComment := j.buildJobResultComment(jobStatus)

	if oldSha != newSha {
		// override the old comment
		return fmt.Sprintf(j.JobsResultNotification, jobComment, newSha)
	}

	jobName := jobStatus.Context
	spliter := "\n"
	js := strings.Split(jobsOldComment, spliter)
	bingo := false
	for i, s := range js {
		m := j.JobResultNotificationRe.FindStringSubmatch(s)
		if m != nil && m[2] == jobName {
			js[i] = jobComment
			bingo = true
			break
		}
	}
	if !bingo {
		js = append(js, jobComment)
	}

	return fmt.Sprintf(j.JobsResultNotification, strings.Join(js, spliter), newSha)
}

func (j *jobStatusComment) parseCombinedStatus(botname, sha string, comments []github.IssueComment) []github.Status {
	jobsComment, oldSha, _ := j.findCheckResultComment(botname, comments)
	if oldSha != sha {
		return []github.Status{}
	}

	js := strings.Split(jobsComment, "\n")
	r := make([]github.Status, 0, len(js))
	for _, s := range js {
		m := j.JobResultNotificationRe.FindStringSubmatch(s)
		if m != nil {
			r = append(r, github.Status{
				State:       reporter.IconToState(m[1]),
				Context:     m[2],
				Description: m[3],
				TargetURL:   m[4],
			})
		}
	}
	return r
}
