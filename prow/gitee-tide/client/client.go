package client

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	githubql "github.com/shurcooL/githubv4"
	"k8s.io/test-infra/prow/gitee"
	tide "k8s.io/test-infra/prow/gitee-tide"
	"k8s.io/test-infra/prow/github"
	reporter "k8s.io/test-infra/prow/job-reporter/gitee"
)

var (
	jobsResultNotification   = "| Tide | Result | Details |\n| --- | --- | --- |\n%s\n  <details>Git tree hash: %s</details>"
	jobsResultNotificationRe = regexp.MustCompile(fmt.Sprintf("\\| Tide \\| Result \\| Details \\|\n\\| --- \\| --- \\| --- \\|\n%s\n  <details>Git tree hash: %s</details>", "([\\s\\S]*)", "(.*)"))
	jobResultNotification    = "| %s %s | %s | [details](%s) |"
	jobResultNotificationRe  = regexp.MustCompile(fmt.Sprintf("\\| %s %s \\| %s \\| \\[details\\]\\(%s\\) \\|", "(.*)", "(.*)", "(.*)", "(.*)"))
)

type giteeClient interface {
	BotName() (string, error)
	ListPRComments(org, repo string, number int) ([]sdk.PullRequestComments, error)
	CreatePRComment(org, repo string, number int, comment string) error
	UpdatePRComment(org, repo string, commentID int, comment string) error
	GetPullRequestChanges(org, repo string, number int) ([]github.PullRequestChange, error)
	GetRef(string, string, string) (string, error)
	GetGiteeRepo(org, repo string) (sdk.Project, error)
	MergePR(owner, repo string, number int, opt sdk.PullRequestMergePutParam) error
	GetPullRequests(org, repo string, opts gitee.ListPullRequestOpt) ([]sdk.PullRequest, error)
}

type ghclient struct {
	giteeClient
}

func NewClient(gec giteeClient) *ghclient {
	return &ghclient{gec}
}
func (c *ghclient) GetRepo(owner, name string) (github.FullRepo, error) {
	_, err := c.GetGiteeRepo(owner, name)
	if err != nil {
		return github.FullRepo{}, err
	}

	r := github.FullRepo{
		AllowMergeCommit: true,
		AllowSquashMerge: true,
		AllowRebaseMerge: false,
	}
	return r, nil
}

func (c *ghclient) Merge(org, repo string, number int, detail github.MergeDetails) error {
	opt := sdk.PullRequestMergePutParam{
		MergeMethod: detail.MergeMethod,
		Title:       detail.CommitTitle,
		Description: detail.CommitMessage,
	}
	return c.MergePR(org, repo, number, opt)
}

func (c *ghclient) GetCombinedStatus(org, repo, ref string, prNumber int) (*github.CombinedStatus, error) {
	comments, err := c.listIssueComments(org, repo, prNumber)
	if err != nil {
		return nil, err
	}

	botname, err := c.BotName()
	if err != nil {
		return nil, err
	}

	jobStatus := reporter.ParseCombinedStatus(botname, ref, comments)

	jsc := jobStatusComment{
		JobsResultNotification:   jobsResultNotification,
		JobsResultNotificationRe: jobsResultNotificationRe,
		JobResultNotification:    jobResultNotification,
		JobResultNotificationRe:  jobResultNotificationRe,
	}
	tideStatus := jsc.parseCombinedStatus(botname, ref, comments)
	if len(tideStatus) == 1 {
		jobStatus = append(jobStatus, tideStatus[0])
	}

	return &github.CombinedStatus{Statuses: jobStatus}, nil
}

func (c *ghclient) CreateStatus(org, repo, ref string, prNumber int, status github.Status) error {
	comments, err := c.listIssueComments(org, repo, prNumber)
	if err != nil {
		return err
	}

	botname, err := c.BotName()
	if err != nil {
		return err
	}

	jsc := jobStatusComment{
		JobsResultNotification:   jobsResultNotification,
		JobsResultNotificationRe: jobsResultNotificationRe,
		JobResultNotification:    jobResultNotification,
		JobResultNotificationRe:  jobResultNotificationRe,
	}
	// find the old comment even if it is not for the current commit in order to
	// write the comment at the fixed position.
	jobsOldComment, oldSha, commentId := jsc.findCheckResultComment(botname, comments)

	desc := jsc.genJobResultComment(jobsOldComment, oldSha, ref, status)

	// oldSha == "" means there is not status comment exist.
	if oldSha == "" {
		return c.CreatePRComment(org, repo, prNumber, desc)
	}
	return c.UpdatePRComment(org, repo, commentId, desc)
}

func (c *ghclient) listIssueComments(org, repo string, number int) ([]github.IssueComment, error) {
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

func (c *ghclient) Query(_ context.Context, r interface{}, vars map[string]interface{}) error {
	q, ok := vars["query"]
	if !ok {
		return fmt.Errorf("Query, can't parse parameter of 'query'")
	}

	q1, ok := q.(githubql.String)
	if !ok {
		return fmt.Errorf("Query, can't convert q(%v) to githubql.String", q)
	}

	isPR, tideQuery, prOpt := parseQueryStr(string(q1))
	if isPR {
		prs, err := c.searchPR(tideQuery, prOpt)
		if err != nil {
			return err
		}
		tide.ConvertToSearchPR(r, prs)
	}

	return nil
}
