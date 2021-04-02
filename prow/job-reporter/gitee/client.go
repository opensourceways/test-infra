package gitee

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	sdk "gitee.com/openeuler/go-gitee/gitee"

	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/github/report"
)

type giteeClient interface {
	BotName() (string, error)
	ListPRComments(org, repo string, number int) ([]sdk.PullRequestComments, error)
	CreatePRComment(org, repo string, number int, comment string) error
	DeletePRComment(org, repo string, ID int) error
	UpdatePRComment(org, repo string, commentID int, comment string) error
	GetGiteePullRequest(org, repo string, number int) (sdk.PullRequest, error)
	ReplacePRAllLabels(owner, repo string, number int, labels []string) error
	AddPRLabel(org, repo string, number int, label string) error
	RemovePRLabel(org, repo string, number int, label string) error
}

var _ report.GitHubClient = (*ghclient)(nil)

type ghclient struct {
	giteeClient
	prNumber int
	botname  string
	baseSHA  string
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
		if prNumber, err = parsePRNumber(org, repo, s); err != nil {
			return err
		}
	}

	h, err := newHelper(c, org, repo, prNumber)
	if err != nil {
		return err
	}

	newComment, newLabel, invalidLabels := h.genCommentAndLabels(c.baseSHA, ref, &s)
	if newComment == "" {
		return nil
	}

	if h.commentID() < 0 {
		err = c.CreatePRComment(org, repo, prNumber, newComment)
	} else {
		err = c.UpdatePRComment(org, repo, h.commentID(), newComment)
	}

	var err1 error
	if newLabel != "" {
		err1 = c.AddPRLabel(org, repo, prNumber, newLabel)
	}

	var err2 error
	for _, l := range invalidLabels {
		err2 = c.RemovePRLabel(org, repo, prNumber, l)
	}

	if err != nil || err1 != nil || err2 != nil {
		return fmt.Errorf(
			"Failed to report job status, write comment error: %v, add label error: %v, remove labels error: %v",
			err, err1, err2)
	}
	return nil
}

func parsePRNumber(org, repo string, s github.Status) (int, error) {
	re := regexp.MustCompile(fmt.Sprintf("http.*/%s_%s/(.*)/%s/.*", org, repo, s.Context))
	m := re.FindStringSubmatch(s.TargetURL)
	if m != nil {
		return strconv.Atoi(m[1])
	}
	return 0, fmt.Errorf("Can't parse pr number from url:%s", s.TargetURL)
}
