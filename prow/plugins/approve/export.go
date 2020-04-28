package approve

import (
	"k8s.io/test-infra/prow/github"
)

func NewState(org, repo, branch, body, author, url string, number int, assignees []github.User) *state {
	return &state{
		org:       org,
		repo:      repo,
		branch:    branch,
		number:    number,
		body:      body,
		author:    author,
		assignees: assignees,
		htmlURL:   url,
	}
}

var (
	Handle = handle
	HelpProvider = helpProvider
)
