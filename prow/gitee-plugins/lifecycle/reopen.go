package lifecycle

import (
	"regexp"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	plugins "k8s.io/test-infra/prow/gitee-plugins"
	originp "k8s.io/test-infra/prow/plugins"
)

var reopenRe = regexp.MustCompile(`(?mi)^/reopen\s*$`)

type reopenClient interface {
	IsCollaborator(owner, repo, login string) (bool, error)
	CreateGiteeIssueComment(org, repo string, number string, comment string) error
	ReopenIssue(owner, repo string, number string) error
}

func reopenIssue(gc reopenClient, log *logrus.Entry, e *sdk.NoteEvent) error {

	org, repo := plugins.GetOwnerAndRepoByEvent(e)
	commentAuthor := e.Comment.User.Login
	author := e.Issue.User.Login
	number := e.Issue.Number

	if !isAuthorOrCollaborator(org,repo,author,commentAuthor,gc.IsCollaborator,log){
		response := "You can't reopen an issue unless you are the author of it or a collaborator."
		return gc.CreateGiteeIssueComment(
			org, repo, number, originp.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, response))
	}

	if err := gc.ReopenIssue(org, repo, number); err != nil {
		return err
	}
	// Add a comment after reopening the issue to leave an audit trail of who
	// asked to reopen it.
	return gc.CreateGiteeIssueComment(
		org, repo, number, originp.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, "Reopened this issue."))
}
