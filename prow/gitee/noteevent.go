package gitee

import sdk "gitee.com/openeuler/go-gitee/gitee"

const (
	//StatusOpen gitee issue or pr status is open
	StatusOpen = "open"
	//StatusOpen gitee issue or pr status is closed
	StatusClosed = "closed"
)

type NoteEvent sdk.NoteEvent

//IsPullRequest Determine whether it is a PullRequest
func (ne *NoteEvent) IsPullRequest() bool {
	return *(ne.NoteableType) == "PullRequest"
}

//IsIssue Determine whether it is a issue
func (ne *NoteEvent) IsIssue() bool {
	return *(ne.NoteableType) == "Issue"
}

//IsCreateCommentEvent Determine whether an note event is create a comment
func (ne *NoteEvent) IsCreateCommentEvent() bool {
	return *(ne.Action) == "comment"
}

//IssueIsClosed whether the status is close  of the issue
func (ne *NoteEvent) IssueIsClosed() bool {
	return ne.Issue.State == StatusClosed
}

//IssueIsOpen whether the status is open  of the issue
func (ne *NoteEvent) IssueIsOpen() bool {
	return ne.Issue.State == StatusOpen
}

//IssueIsOpen whether the status is open  of the PullRequest
func (ne *NoteEvent) PRIsOpen() bool {
	return ne.PullRequest.State == StatusOpen
}

//CommenterIsIssueAuthor Whether the author of the comment is the author of the issue
func (ne *NoteEvent) CommenterIsIssueAuthor() bool {
	return ne.Comment.User.Login == ne.Issue.User.Login
}

//CommenterIsPRAuthor Whether the author of the comment is the author of the PR
func (ne *NoteEvent) CommenterIsPRAuthor() bool {
	return ne.Comment.User.Login == ne.PullRequest.User.Login
}

//CommenterIsAuthor Whether the author of the comment is the author of the PR or issue
func (ne *NoteEvent) CommenterIsAuthor() bool {
	if ne.IsIssue() {
		return ne.CommenterIsIssueAuthor()
	}
	if ne.IsPullRequest() {
		return ne.CommenterIsPRAuthor()
	}
	return false
}

//GetCommenter Return to the author of the comment
func (ne *NoteEvent) GetCommenter() string {
	return ne.Comment.User.Login
}

//GetPRNumber Return to the number of the PR
func (ne *NoteEvent) GetPRNumber() int {
	return int(ne.PullRequest.Number)
}

//GetIssueNumber Return to the number of the issue
func (ne *NoteEvent) GetIssueNumber() string {
	return ne.Issue.Number
}
