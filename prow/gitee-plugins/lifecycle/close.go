package lifecycle

import (
	"fmt"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/gitee"
	giteep "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/plugins"
)

func closeIssue(gc giteeClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	org, repo := giteep.GetOwnerAndRepoByEvent(e)
	ne := (*gitee.NoteEvent)(e)
	commentAuthor := ne.GetCommenter()
	number := ne.GetIssueNumber()

	if !isAuthorOrCollaborator(org, repo, ne, gc, log) {
		response := "You can't close an issue unless you are the author of it or a collaborator."
		return gc.CreateGiteeIssueComment(
			org, repo, number, plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, response))
	}

	if err := gc.CloseIssue(org, repo, number); err != nil {
		return fmt.Errorf("error close issue:%v", err)
	}

	response := plugins.FormatResponseRaw(e.Comment.Body, e.Comment.HtmlUrl, commentAuthor, "Closed this issue.")
	return gc.CreateGiteeIssueComment(org, repo, number, response)
}

func closePullRequest(gc giteeClient, log *logrus.Entry, e *sdk.NoteEvent) error {
	ne := (*gitee.NoteEvent)(e)
	if !ne.PRIsOpen() || !closeRe.MatchString(e.Comment.Body) {
		return nil
	}

	org, repo := giteep.GetOwnerAndRepoByEvent(e)
	commentAuthor := ne.GetCommenter()
	number := ne.GetPRNumber()

	if !isAuthorOrCollaborator(org, repo, ne, gc, log) {
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

func isAuthorOrCollaborator(org, repo string, ne *gitee.NoteEvent, gh giteeClient, log *logrus.Entry) bool {
	if ne.CommenterIsAuthor() {
		return true
	}
	commenter := ne.GetCommenter()
	isCollaborator, err := gh.IsCollaborator(org, repo, commenter)
	if err != nil {
		log.WithError(err).Errorf("Failed IsCollaborator(%s, %s, %s)", org, repo, commenter)
	}
	return isCollaborator
}
