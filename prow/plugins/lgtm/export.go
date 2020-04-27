package lgtm

import "k8s.io/test-infra/prow/github"

func NewReviewCtx(author, issueAuthor, body, htmlURL string, repo github.Repo, assignees []github.User, number int) reviewCtx {
	return reviewCtx{
		author:      author,
		issueAuthor: issueAuthor,
		body:        body,
		htmlURL:     htmlURL,
		repo:        repo,
		assignees:   assignees,
		number:      number,
	}
}

var (
	Handle = handle
	HelpProvider = helpProvider
	HandlePullRequest = handlePullRequest
)
