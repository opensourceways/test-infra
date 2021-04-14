package lifecycle

import (
	"fmt"
	"regexp"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

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

func handleClose(gc closeClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	if !closeRe.MatchString(e.Comment.Body) {
		return nil
	}
	if isPr(*e.NoteableType) {
		return closePullRequest(gc, log, e)
	}
	return closeIssue(gc, log, e)
}

func closeIssue(gc closeClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	if e.Issue.State != "open" {
		return nil
	}
	org := e.Repository.Namespace
	repo := e.Repository.Path
	number := e.Issue.Number
	commentAuthor := e.Comment.User.Login

	isAuthor := e.Issue.User.Login == commentAuthor

	isCollaborator, err := gc.IsCollaborator(org, repo, commentAuthor)
	if err != nil {
		log.WithError(err).Errorf("Failed IsCollaborator(%s, %s, %s)", org, repo, commentAuthor)
	}
	// Only authors and collaborators are allowed to close  issues.
	if !isAuthor && !isCollaborator {
		response := "You can't close an  issue unless you authored it or you are a collaborator."
		log.Infof("Commenting \"%s\".", response)
		return gc.CreateGiteeIssueComment(
			org, repo, number, plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, response))
	}
	if err := gc.CloseIssue(org, repo, number); err != nil {
		return fmt.Errorf("error close issue:%v", err)
	}
	return nil
}

func closePullRequest(gc closeClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	if e.PullRequest.State != "open" {
		return nil
	}
	org := e.Repository.Namespace
	repo := e.Repository.Path
	number := int(e.PullRequest.Number)
	commentAuthor := e.Comment.User.Login

	isAuthor := e.PullRequest.User.Login == commentAuthor

	isCollaborator, err := gc.IsCollaborator(org, repo, commentAuthor)
	if err != nil {
		log.WithError(err).Errorf("Failed IsCollaborator(%s, %s, %s)", org, repo, commentAuthor)
	}

	// Only authors and collaborators are allowed to close  PR.
	if !isAuthor && !isCollaborator {
		response := "You can't close an  PullRequest unless you authored it or you are a collaborator."
		log.Infof("Commenting \"%s\".", response)
		return gc.CreatePRComment(
			org, repo, number, plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, response))
	}
	if err := gc.ClosePR(org, repo, number); err != nil {
		return fmt.Errorf("Error closing PR: %v ", err)
	}
	response := plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, "Closed this PR.")
	return gc.CreatePRComment(org, repo, number, response)
}
