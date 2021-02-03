package associate

import (
	"fmt"
	"regexp"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
)

const (
	missIssueComment = "@%s PullRequest must be associated with an issue, please associate at least one issue. after associating an issue, you can use the **/check-issue** command to remove the **miss/issue** label."
	missIssueLabel   = "miss/issue"
)

var (
	checkIssueRe    = regexp.MustCompile(`(?mi)^/check-issue\s*$`)
	removeMissIssue = regexp.MustCompile(`(?mi)^/remove-miss/issue\s*$`)
)

type checkIssueClient interface {
	AddPRLabel(org, repo string, number int, label string) error
	RemovePRLabel(org, repo string, number int, label string) error
	CreatePRComment(org, repo string, number int, comment string) error
	IsCollaborator(owner, repo, login string) (bool, error)
	ListPrIssues(org, repo string, number int32) ([] sdk.Issue, error)
}

func handlePrComment(ghc checkIssueClient, e *sdk.NoteEvent) error {
	if checkIssueRe.MatchString(e.Comment.Body) {
		return handleCheckIssue(ghc, e)
	}
	if removeMissIssue.MatchString(e.Comment.Body) {
		return handleRemoveMissLabel(ghc, e)
	}
	return nil
}

func handleRemoveMissLabel(ghc checkIssueClient, e *sdk.NoteEvent) error {
	org := e.Repository.Namespace
	repo := e.Repository.Path
	number := int(e.PullRequest.Number)
	if hasLabel := judgeHasLabel(e.PullRequest.Labels,missIssueLabel); !hasLabel {
		return nil
	}
	author := e.Comment.User.Login
	isCo, err := ghc.IsCollaborator(org, repo, author)
	if err != nil {
		return err
	}
	if !isCo {
		comment := fmt.Sprintf("@%s members of the project maintainer gitee team can use the '/remove-miss/issue' command.", author)
		return ghc.CreatePRComment(org, repo, number, comment)
	}
	return ghc.RemovePRLabel(org, repo, int(e.PullRequest.Number), missIssueLabel)
}

func handleCheckIssue(ghc checkIssueClient, e *sdk.NoteEvent) error {
	org := e.Repository.Namespace
	repo := e.Repository.Path
	number := e.PullRequest.Number
	author := e.PullRequest.User.Login
	issues, err := ghc.ListPrIssues(org, repo, number)
	if err != nil {
		return err
	}
	hasLabel := judgeHasLabel(e.PullRequest.Labels,missIssueLabel)
	if len(issues) == 0 {
		if hasLabel {
			return nil
		}
		err = ghc.AddPRLabel(org, repo, int(number), missIssueLabel)
		if err != nil {
			return err
		}
		return ghc.CreatePRComment(org, repo, int(number), fmt.Sprintf(missIssueComment, author))
	}
	if !hasLabel {
		return nil
	}
	return ghc.RemovePRLabel(org, repo, int(number), missIssueLabel)

}

func handlePrCreate(ghc checkIssueClient, e *sdk.PullRequestEvent, log *logrus.Entry) error {
	if *(e.Action) != "open" {
		return nil
	}
	org := e.Repository.Namespace
	repo := e.Repository.Path
	number := e.PullRequest.Number
	author := e.PullRequest.User.Login
	iss, err := ghc.ListPrIssues(org, repo, number)
	if err != nil {
		log.Debug("get pr issues fail.")
		return err
	}
	if len(iss) == 0 && !judgeHasLabel(e.PullRequest.Labels,missIssueLabel){
		err = ghc.AddPRLabel(org, repo, int(number), missIssueLabel)
		if err != nil {
			return err
		}
		return ghc.CreatePRComment(org, repo, int(number), fmt.Sprintf(missIssueComment, author))
	}
	return nil
}

