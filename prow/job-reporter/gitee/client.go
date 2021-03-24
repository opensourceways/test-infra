package gitee

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/github/report"
)

var (
	jobsResultNotification   = "| Check Name | Result | Details |\n| --- | --- | --- |\n%s\n  <details>Git tree hash: %s</details>"
	jobsResultNotificationRe = regexp.MustCompile(fmt.Sprintf("\\| Check Name \\| Result \\| Details \\|\n\\| --- \\| --- \\| --- \\|\n%s\n  <details>Git tree hash: %s</details>", "([\\s\\S]*)", "(.*)"))
	jobResultNotification    = "| %s %s | %s | [details](%s) |"
	jobResultEachPartRe      = regexp.MustCompile(fmt.Sprintf("\\| %s %s \\| %s \\| \\[details\\]\\(%s\\) \\|", "(.*)", "(.*)", "(.*)", "(.*)"))
	jobStatusLabelRe         = regexp.MustCompile(`^ci/test-(error|failure|pending|success)$`)
)

type giteeClient interface {
	BotName() (string, error)
	ListPRComments(org, repo string, number int) ([]sdk.PullRequestComments, error)
	CreatePRComment(org, repo string, number int, comment string) error
	DeletePRComment(org, repo string, ID int) error
	UpdatePRComment(org, repo string, commentID int, comment string) error
	GetGiteePullRequest(org, repo string, number int) (sdk.PullRequest, error)
	ReplacePRAllLabels(owner, repo string, number int, labels []string) error
}

var _ report.GitHubClient = (*ghclient)(nil)

type ghclient struct {
	giteeClient
	prNumber int
}

func (c *ghclient) ListIssueComments(org, repo string, number int) ([]github.IssueComment, error) {
	var r []github.IssueComment

	v, err := c.ListPRComments(org, repo, number)
	if err != nil {
		return r, err
	}

	for _, i := range v {
		r = append(r, gitee.ConvertGiteePRComment(i))
	}

	sort.SliceStable(r, func(i, j int) bool {
		return r[i].CreatedAt.Before(r[j].CreatedAt)
	})

	return r, nil
}

func (c *ghclient) CreateComment(owner, repo string, number int, comment string) error {
	return c.CreatePRComment(owner, repo, number, comment)
}

func (c *ghclient) DeleteComment(org, repo string, id int) error {
	return c.DeletePRComment(org, repo, id)
}

func (c *ghclient) EditComment(org, repo string, ID int, comment string) error {
	return c.UpdatePRComment(org, repo, ID, comment)
}

func (c *ghclient) CreateStatus(org, repo, ref string, s github.Status) error {
	prNumber := c.prNumber
	var err error
	if prNumber <= 0 {
		prNumber, err = parsePRNumber(org, repo, s)
		if err != nil {
			return err
		}
	}

	pr, err := c.GetGiteePullRequest(org, repo, prNumber)
	if err != nil {
		return err
	}
	if ref != pr.Head.Sha {
		// Secondly check whether the status is for the newest commit, if not, skip.
		// This check is for the case that two updates for pr happend very closely.
		return nil
	}

	comments, err := c.ListIssueComments(org, repo, prNumber)
	if err != nil {
		return err
	}

	botname, err := c.BotName()
	if err != nil {
		return err
	}

	jsc := JobStatusComment{
		JobsResultNotification:   jobsResultNotification,
		JobsResultNotificationRe: jobsResultNotificationRe,
		JobResultNotification:    jobResultNotification,
		JobResultNotificationRe:  jobResultEachPartRe,
	}
	// find the old comment even if it is not for the current commit in order to
	// write the comment at the fixed position.
	jobsOldComment, oldSha, commentID := jsc.FindCheckResultComment(botname, comments)

	desc := jsc.GenJobResultComment(jobsOldComment, oldSha, ref, s)
	status := jsc.parseCommentToJobStatus(desc)

	uErr := c.updatePRLabel(org, repo, int32(prNumber), pr.Labels, status)
	// oldSha == "" means there is not status comment exist.
	if oldSha == "" {
		err = c.CreatePRComment(org, repo, prNumber, desc)
	}else {
		err = c.UpdatePRComment(org, repo, commentID, desc)
	}
	if uErr != nil || err != nil {
		return fmt.Errorf("report job status label or comment error, label error: %v; comment error: %v", uErr, err)
	}
	return nil
}

func (c *ghclient) updatePRLabel(org, repo string, number int32, labels []sdk.Label, status []github.Status) error {
	labelSet := sets.String{}
	for _, v := range labels {
		if !jobStatusLabelRe.MatchString(v.Name) {
			labelSet.Insert(v.Name)
		}
	}
	statusSet := sets.String{}
	for _, s := range status {
		statusSet.Insert(s.State)
	}
	labelSet.Insert(genLabelByJobStatus(statusSet))
	return c.ReplacePRAllLabels(org, repo, int(number), labelSet.List())
}

func genLabelByJobStatus(statusSet sets.String) string {
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

func parsePRNumber(org, repo string, s github.Status) (int, error) {
	re := regexp.MustCompile(fmt.Sprintf("http.*/%s_%s/(.*)/%s/.*", org, repo, s.Context))
	m := re.FindStringSubmatch(s.TargetURL)
	if m != nil {
		return strconv.Atoi(m[1])
	}
	return 0, fmt.Errorf("Can't parse pr number from url:%s", s.TargetURL)
}

func ParseCombinedStatus(botname, sha string, comments []github.IssueComment) []github.Status {
	jsc := JobStatusComment{
		JobsResultNotification:   jobsResultNotification,
		JobsResultNotificationRe: jobsResultNotificationRe,
		JobResultNotification:    jobResultNotification,
		JobResultNotificationRe:  jobResultEachPartRe,
	}
	return jsc.ParseCombinedStatus(botname, sha, comments)
}
