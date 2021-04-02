package gitee

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/test-infra/prow/github"
)

const spliter = "\n"

type JobsComment struct {
	JobsResultNotificationRe *regexp.Regexp

	JobComment JobComment
}

func (j *JobsComment) FindComment(botname string, comments []github.IssueComment) *github.IssueComment {
	for i := len(comments) - 1; i >= 0; i-- {
		comment := &comments[i]
		if comment.User.Login != botname {
			continue
		}

		if j.JobsResultNotificationRe.Match([]byte(comment.Body)) {
			return comment
		}
	}

	return nil
}

func (j *JobsComment) UpdateComment(comment string, status *github.Status) string {
	js := func() string {
		return j.JobComment.GenJobComment(status)
	}

	if comment == "" {
		return js()
	}

	cs := strings.Split(comment, spliter)
	for i, item := range cs {
		s1, err := j.JobComment.ParseJobComment(item)
		if err == nil && s1.Context == status.Context {
			cs[i] = js()
			return strings.Join(cs, spliter)
		}
	}

	return fmt.Sprintf("%s%s%s", comment, spliter, js())
}

func (j *JobsComment) ParseComment(comment string) []github.Status {
	if comment == "" {
		return nil
	}

	cs := strings.Split(comment, spliter)
	r := make([]github.Status, 0, len(cs))
	for _, item := range cs {
		if job, err := j.JobComment.ParseJobComment(item); err == nil {
			r = append(r, job)
		}
	}
	return r
}
