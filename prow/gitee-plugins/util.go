package plugins

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"

	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/github"
)

var (
	emailRe = regexp.MustCompile(`[a-zA-Z0-9_.-]+@[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)*\.[a-zA-Z]{2,6}`)
)

func NoteEventToCommentEvent(e *sdk.NoteEvent) github.GenericCommentEvent {
	gc := github.GenericCommentEvent{
		Repo: github.Repo{
			Owner: github.User{
				Login: e.Repository.Namespace,
			},
			Name: e.Repository.Path,
		},
		User: github.User{
			Login: e.Comment.User.Login,
		},
		Action:  convertNoteEventAction(e),
		Body:    e.Comment.Body,
		HTMLURL: e.Comment.HtmlUrl,
		GUID:    "", //TODO
	}

	switch *(e.NoteableType) {
	case "PullRequest":
		setPullRequestInfo(e, &gc)
	}

	return gc
}

func convertNoteEventAction(e *sdk.NoteEvent) github.GenericCommentEventAction {
	var a github.GenericCommentEventAction

	switch *(e.Action) {
	case "comment":
		a = github.GenericCommentActionCreated
	}
	return a
}

func convertAssignees(assignees []sdk.UserHook) []github.User {
	r := make([]github.User, len(assignees))
	for i, item := range assignees {
		r[i] = github.User{Login: item.Login}
	}
	return r
}

func setPullRequestInfo(e *sdk.NoteEvent, gc *github.GenericCommentEvent) {
	pr := e.PullRequest
	gc.IsPR = true
	gc.IssueState = pr.State
	gc.IssueAuthor.Login = pr.User.Login
	gc.Number = int(pr.Number)
	gc.IssueBody = pr.Body
	gc.IssueHTMLURL = pr.HtmlUrl
	gc.Assignees = convertAssignees(pr.Assignees)
}

func ConvertPullRequestEvent(e *sdk.PullRequestEvent) github.PullRequestEvent {
	epr := e.PullRequest
	pe := github.PullRequestEvent{
		Action: ConvertPullRequestAction(e),
		GUID:   "", //TODO
		PullRequest: github.PullRequest{
			Base: github.PullRequestBranch{
				Repo: github.Repo{
					Name: epr.Base.Repo.Path,
					Owner: github.User{
						Login: epr.Base.Repo.Namespace,
					},
					HTMLURL:  epr.Base.Repo.HtmlUrl,
					FullName: epr.Base.Repo.FullName,
				},
				Ref: epr.Base.Ref,
				SHA: epr.Base.Sha,
			},
			Head: github.PullRequestBranch{
				SHA: epr.Head.Sha,
			},
			User: github.User{
				Login:   epr.User.Login,
				HTMLURL: epr.User.HtmlUrl,
			},
			Number:   int(epr.Number),
			HTMLURL:  epr.HtmlUrl,
			State:    epr.State,
			Body:     epr.Body,
			Title:    epr.Title,
			Labels:   convertPullRequestLabel(e),
			ID:       int(epr.Id),
			Mergable: &(epr.Mergeable),
		},
		// Label:
	}

	return pe
}

func ConvertPushEvent(e *sdk.PushEvent) github.PushEvent {
	pe := github.PushEvent{
		GUID:    "", //TODO
		Ref:     *(e.Ref),
		Deleted: *(e.Deleted),
		After:   *(e.After),
		Repo: github.Repo{
			Owner: github.User{
				Login: e.Repository.Namespace,
			},
			Name: e.Repository.Path,
		},
		Commits: convertPushCommits(e),
		Compare: *(e.Compare),
	}
	return pe
}

func convertPushCommits(e *sdk.PushEvent) []github.Commit {
	r := make([]github.Commit, 0, len(e.Commits))
	for _, i := range e.Commits {
		r = append(r, github.Commit{
			Added:    i.Added,
			Removed:  i.Removed,
			Modified: i.Modified,
			ID:       i.Id,
			Message:  i.Message,
		})
	}
	return r
}

func ConvertPullRequestAction(e *sdk.PullRequestEvent) github.PullRequestEventAction {
	var a github.PullRequestEventAction

	switch strings.ToLower(*(e.Action)) {
	case "open":
		a = github.PullRequestActionOpened
	case "update":
		switch strings.ToLower(*(e.ActionDesc)) {
		case "source_branch_changed": // change the pr's commits
			a = github.PullRequestActionSynchronize
		case "target_branch_changed": // change the branch to which this pr will be merged
			a = github.PullRequestActionEdited
		case "update_label":
			a = github.PullRequestActionLabeled
		}
	case "close":
		a = github.PullRequestActionClosed
	}

	return a
}

func convertPullRequestLabel(e *sdk.PullRequestEvent) []github.Label {
	r := make([]github.Label, 0, len(e.PullRequest.Labels))

	for _, i := range e.PullRequest.Labels {
		r = append(r, github.Label{Name: i.Name})
	}
	return r
}

func checkNoteEvent(e *sdk.NoteEvent) error {
	eventType := "note event"
	ne := gitee.NewNoteEventWrapper(e)
	if ne.Comment == nil {
		return fmtCheckError(eventType, "comment")
	}
	if ne.IsPullRequest() {
		if err := checkPullRequestHook(ne.PullRequest, eventType); err != nil {
			return err
		}
	}
	if ne.IsIssue() && ne.Issue == nil {
		return fmtCheckError(eventType, "issue")
	}
	return checkRepository(e.Repository, eventType)
}

func checkIssueEvent(e *sdk.IssueEvent) error {
	eventType := "issue event"
	if e.Issue == nil {
		return fmtCheckError(eventType, "issue")
	}
	return checkRepository(e.Repository, eventType)
}

func checkPullRequestEvent(e *sdk.PullRequestEvent) error {
	eventType := "pull request event"
	if err := checkPullRequestHook(e.PullRequest, eventType); err != nil {
		return err
	}
	return checkRepository(e.Repository, eventType)
}

func checkPullRequestHook(pr *sdk.PullRequestHook, eventType string) error {
	if pr == nil {
		return fmtCheckError(eventType, "pull_request")
	}
	if pr.Head == nil || pr.Base == nil {
		return fmtCheckError(eventType, "pull_request.head or pull_request.base")
	}
	return nil
}

func checkRepository(rep *sdk.ProjectHook, eventType string) error {
	if rep == nil {
		return fmtCheckError(eventType, "pull_request")
	}
	if rep.Namespace == "" || rep.Path == "" {
		return fmtCheckError(eventType, "pull_request.namespace or pull_request.path")
	}
	return nil
}

func fmtCheckError(eventType, field string) error {
	return fmt.Errorf("%s is illegal: the %s field is empty", eventType, field)
}

func NormalEmail(email string) string {
	v := emailRe.FindStringSubmatch(email)
	if len(v) > 0 {
		return v[0]
	}
	return email
}

type BotComment struct {
	CommentID int
	Body      string
	CreatedAt time.Time
}

func (c BotComment) Exists() bool {
	return c.Body != ""
}

func FindBotComment(allComments []sdk.PullRequestComments, botName string, re *regexp.Regexp) []BotComment {
	r := []BotComment{}
	for i := range allComments {
		item := &allComments[i]

		if item.User == nil || item.User.Login != botName {
			continue
		}

		if re.MatchString(item.Body) {
			ut, err := time.Parse(time.RFC3339, item.UpdatedAt)
			if err != nil {
				// it is a invalid comment if parsing time failed
				continue
			}
			r = append(r, BotComment{
				CommentID: int(item.Id),
				Body:      item.Body,
				CreatedAt: ut,
			})
		}
	}
	return r
}
