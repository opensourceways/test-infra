package lifecycle

import (
	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/gitee"
	giteep "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/plugins"
)

func reopenIssue(gc giteeClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	org, repo := giteep.GetOwnerAndRepoByEvent(e)
	ne := (*gitee.NoteEvent)(e)
	commentAuthor := ne.GetCommenter()
	number := ne.GetIssueNumber()

	if !isAuthorOrCollaborator(org, repo, ne, gc, log) {
		response := "You can't reopen an issue unless you are the author of it or a collaborator."
		return gc.CreateGiteeIssueComment(
			org, repo, number, plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, response))
	}

	if err := gc.ReopenIssue(org, repo, number); err != nil {
		return err
	}
	// Add a comment after reopening the issue to leave an audit trail of who
	// asked to reopen it.
	return gc.CreateGiteeIssueComment(
		org, repo, number, plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, "Reopened this issue."))
}
