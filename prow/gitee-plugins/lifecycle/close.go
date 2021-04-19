package lifecycle

import (
	"fmt"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/plugins"
)

func closeIssue(gc giteeClient, log *logrus.Entry, ne gitee.IssueNoteEvent) error {
	org, repo := ne.GetOrgRep()
	commenter := ne.GetCommenter()
	number := ne.GetIssueNumber()

	if ne.GetIssueAuthor() != commenter && !isCollaborator(org, repo, commenter, gc, log) {
		resp := response(
			ne.NoteEventWrapper,
			"You can't close an issue unless you are the author of it or a collaborator.",
		)
		return gc.CreateIssueComment(org, repo, number, resp)
	}

	if err := gc.CloseIssue(org, repo, number); err != nil {
		return fmt.Errorf("error close issue:%v", err)
	}

	return gc.CreateIssueComment(
		org, repo, number, response(ne.NoteEventWrapper, "Closed this issue."),
	)
}

func closePullRequest(gc giteeClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	ne := gitee.NewPRNoteEvent(e)
	if !ne.IsPROpen() || !closeRe.MatchString(ne.GetComment()) {
		return nil
	}

	org, repo := ne.GetOrgRep()
	commenter := ne.GetCommenter()
	number := ne.GetPRNumber()

	if ne.GetPRAuthor() != commenter && !isCollaborator(org, repo, commenter, gc, log) {
		resp := response(
			ne.NoteEventWrapper,
			"You can't close a pull reuqest unless you are the author of it or a collaborator.",
		)
		return gc.CreatePRComment(org, repo, number, resp)
	}

	if err := gc.ClosePR(org, repo, number); err != nil {
		return fmt.Errorf("Error closing PR: %v ", err)
	}

	return gc.CreatePRComment(org, repo, number, response(ne.NoteEventWrapper, "Closed this PR."))
}

func isCollaborator(org, repo, commenter string, gh giteeClient, log *logrus.Entry) bool {
	isCollaborator, err := gh.IsCollaborator(org, repo, commenter)
	if err != nil {
		log.WithError(err).Errorf("Failed IsCollaborator(%s, %s, %s)", org, repo, commenter)
	}
	return isCollaborator
}

func response(e gitee.NoteEventWrapper, desc string) string {
	return plugins.FormatResponseRaw(e.GetComment(), e.Comment.HtmlUrl, e.GetCommenter(), desc)
}
