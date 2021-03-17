package gitee

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/test-infra/prow/github"
)

type JobStatusComment struct {
	JobsResultNotification   string
	JobsResultNotificationRe *regexp.Regexp
	JobResultNotification    string
	JobResultNotificationRe  *regexp.Regexp
}

func (j *JobStatusComment) FindCheckResultComment(botname string, comments []github.IssueComment) (string, string, int) {
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

func (j *JobStatusComment) buildJobResultComment(s github.Status) string {
	icon := stateToIcon(s.State)
	return fmt.Sprintf(j.JobResultNotification, icon, s.Context, s.Description, s.TargetURL)
}

func stateToIcon(state string) string {
	icon := ""
	switch state {
	case github.StatusPending:
		icon = ":large_blue_circle:"
	case github.StatusSuccess:
		icon = ":white_check_mark:"
	case github.StatusFailure:
		icon = ":x:"
	case github.StatusError:
		icon = ":heavy_minus_sign:"
	}
	return icon
}

func iconToState(icon string) string {
	switch icon {
	case ":large_blue_circle:":
		return github.StatusPending
	case ":white_check_mark:":
		return github.StatusSuccess
	case ":x:":
		return github.StatusFailure
	case ":heavy_minus_sign:":
		return github.StatusError
	}
	return ""
}

func (j *JobStatusComment) GenJobResultComment(jobsOldComment, oldSha, newSha string, jobStatus github.Status) string {
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

func (j *JobStatusComment) ParseCombinedStatus(botname, sha string, comments []github.IssueComment) []github.Status {
	jobsComment, oldSha, _ := j.FindCheckResultComment(botname, comments)
	if oldSha != sha {
		return []github.Status{}
	}
	return j.parseCommentToJobStatus(jobsComment)
}

func (j *JobStatusComment) parseCommentToJobStatus(comment string) []github.Status {
	js := strings.Split(comment, "\n")
	r := make([]github.Status, 0, len(js))
	for _, s := range js {
		m := j.JobResultNotificationRe.FindStringSubmatch(s)
		if m != nil {
			r = append(r, github.Status{
				State:       iconToState(m[1]),
				Context:     m[2],
				Description: m[3],
				TargetURL:   m[4],
			})
		}
	}
	return r
}
