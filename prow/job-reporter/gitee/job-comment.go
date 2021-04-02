package gitee

import (
	"fmt"
	"regexp"

	"k8s.io/test-infra/prow/github"
)

var (
	jobResultNotification   = "| %s %s | %s | [details](%s) |"
	jobResultNotificationRe = regexp.MustCompile(fmt.Sprintf("\\| %s %s \\| %s \\| \\[details\\]\\(%s\\) \\|", "(.*)", "(.*)", "(.*)", "(.*)"))
)

type JobComment interface {
	GenJobComment(s *github.Status) string
	ParseJobComment(s string) (github.Status, error)
}

type jobComment struct{}

func (j jobComment) GenJobComment(s *github.Status) string {
	icon := StateToIcon(s.State)
	return fmt.Sprintf(jobResultNotification, icon, s.Context, s.Description, s.TargetURL)
}

func (j jobComment) ParseJobComment(s string) (github.Status, error) {
	m := jobResultNotificationRe.FindStringSubmatch(s)
	if m != nil {
		return github.Status{
			State:       IconToState(m[1]),
			Context:     m[2],
			Description: m[3],
			TargetURL:   m[4],
		}, nil
	}

	return github.Status{}, fmt.Errorf("invalid job comment")
}

func StateToIcon(state string) string {
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

func IconToState(icon string) string {
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
