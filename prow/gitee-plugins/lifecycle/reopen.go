package lifecycle

import (
	"regexp"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/plugins"
)

var reopenRe = regexp.MustCompile(`(?mi)^/reopen\s*$`)

type reopenClient interface {
	IsCollaborator(owner, repo, login string) (bool, error)
	CreateGiteeIssueComment(org, repo string, number string, comment string) error
	ReopenIssue(owner, repo string, number string) error
}

func handleReopen(gc reopenClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	if isPr(*e.NoteableType) || e.Issue.State != "closed" {
		return nil
	}
	if !reopenRe.MatchString(e.Comment.Body) {
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
	// Only authors and collaborators are allowed to reopen issues or PRs.
	if !isAuthor && !isCollaborator {
		response := "You can't reopen an issue/PR unless you authored it or you are a collaborator."
		log.Infof("Commenting \"%s\".", response)
		return gc.CreateGiteeIssueComment(
			org, repo, number, plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, response))
	}
	err = gc.ReopenIssue(org, repo, number)
	if err != nil {
		return err
	}
	// Add a comment after reopening the issue to leave an audit trail of who
	// asked to reopen it.
	return gc.CreateGiteeIssueComment(
		org, repo, number, plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, "Reopened this issue."))
}
