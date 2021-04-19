package lifecycle

import (
	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/gitee"
)

func reopenIssue(gc giteeClient, log *logrus.Entry, ne gitee.IssueNoteEvent) error {
	org, repo := ne.GetOrgRep()
	commenter := ne.GetCommenter()
	number := ne.GetIssueNumber()

	if ne.GetIssueAuthor() != commenter && !isCollaborator(org, repo, commenter, gc, log) {
		resp := response(
			ne.NoteEventWrapper,
			"You can't reopen an issue unless you are the author of it or a collaborator.",
		)
		return gc.CreateIssueComment(org, repo, number, resp)
	}

	if err := gc.ReopenIssue(org, repo, number); err != nil {
		return err
	}

	// Add a comment after reopening the pr to leave an audit trail of who asked to reopen it.
	return gc.CreateIssueComment(org, repo, number, response(ne.NoteEventWrapper, "Reopened this issue."))
}
