package approve

import (
	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/plugins"
	"k8s.io/test-infra/prow/plugins/approve/approvers"
)

func Handle(log *logrus.Entry, ghc githubClient, repo approvers.Repo, githubConfig config.GitHubOptions, opts *plugins.Approve, pr *state) error {
	return handle(log, ghc, repo, githubConfig, opts, pr)
}

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

var HelpProvider = helpProvider
