package gitee

const (
	//StatusOpen gitee issue or pr status is open
	StatusOpen = "open"
	//StatusOpen gitee issue or pr status is closed
	StatusClosed = "closed"

)

//IsPullRequest Determine whether it is a PullRequest
func IsPullRequest(noteType string) bool {
	return noteType == "PullRequest"
}

//IsIssue Determine whether it is a issue
func IsIssue(noteType string) bool {
	return noteType == "issue"
}




