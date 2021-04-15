package lifecycle

import (
	"fmt"
	"regexp"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/gitee"
	giteep "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/plugins"
)

var closeRe = regexp.MustCompile(`(?mi)^/close\s*$`)

type closeClient interface {
	CreatePRComment(owner, repo string, number int, comment string) error
	CreateGiteeIssueComment(owner, repo string, number string, comment string) error
	IsCollaborator(owner, repo, login string) (bool, error)
	CloseIssue(owner, repo string, number string) error
	ClosePR(owner, repo string, number int) error
}

func closeIssue(gc closeClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	org, repo := giteep.GetOwnerAndRepoByEvent(e)
	commentAuthor := e.Comment.User.Login
	author := e.Issue.User.Login
	number := e.Issue.Number

	if !isAuthorOrCollaborator(org, repo, author, commentAuthor, gc.IsCollaborator, log) {
		response := "You can't close an issue unless you are the author of it or a collaborator."
		return gc.CreateGiteeIssueComment(
			org, repo, number, plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, response))
	}

	if err := gc.CloseIssue(org, repo, number); err != nil {
		return fmt.Errorf("error close issue:%v", err)
	}
	return nil
}

func closePullRequest(gc closeClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	if e.PullRequest.State != gitee.StatusOpen || !closeRe.MatchString(e.Comment.Body) {
		return nil
	}
	org, repo := giteep.GetOwnerAndRepoByEvent(e)
	commentAuthor := e.Comment.User.Login
	author := e.PullRequest.User.Login
	number := int(e.PullRequest.Number)

	if !isAuthorOrCollaborator(org, repo, author, commentAuthor, gc.IsCollaborator, log) {
		response := "You can't close an pullreuqest unless you are the author of it or a collaborator"
		return gc.CreatePRComment(
			org, repo, number, plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, response))
	}

	if err := gc.ClosePR(org, repo, number); err != nil {
		return fmt.Errorf("Error closing PR: %v ", err)
	}

	response := plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, "Closed this PR.")
	return gc.CreatePRComment(org, repo, number, response)
}

type collaboratorFunc func(string, string, string) (bool, error)

func isAuthorOrCollaborator(org, repo, author, commenter string, isCollaboratorFunc collaboratorFunc, log *logrus.Entry) bool {
	if author == commenter {
		return true
	}
	isCollaborator, err := isCollaboratorFunc(org, repo, commenter)
	if err != nil {
		log.WithError(err).Errorf("Failed IsCollaborator(%s, %s, %s)", org, repo, commenter)
	}
	return isCollaborator
}
