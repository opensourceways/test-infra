package size

import (
	"encoding/base64"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"k8s.io/test-infra/prow/github"
)

type client interface {
	AddPRLabel(org, repo string, number int, label string) error
	GetPullRequestChanges(org, repo string, number int) ([]github.PullRequestChange, error)
	RemovePRLabel(org, repo string, number int, label string) error
	GetPRLabels(org, repo string, number int) ([]sdk.Label, error)
	GetPathContent(owner, repo, path, ref string) (sdk.Content, error)
}

type clientAdapter struct {
	client
}

func (sca *clientAdapter) AddLabel(owner, repo string, number int, label string) error {
	return sca.AddPRLabel(owner, repo, number, label)
}

func (sca *clientAdapter) RemoveLabel(owner, repo string, number int, label string) error {
	return sca.AddPRLabel(owner, repo, number, label)
}

func (sca *clientAdapter) GetIssueLabels(org, repo string, number int) ([]github.Label, error) {
	var r []github.Label

	v, err := sca.GetPRLabels(org, repo, number)
	if err != nil {
		return r, err
	}

	for _, i := range v {
		r = append(r, github.Label{Name: i.Name})
	}
	return r, nil
}

func (sca *clientAdapter) GetFile(org, repo, filepath, commit string) ([]byte, error) {
	content, err := sca.GetPathContent(org, repo, filepath, commit)
	if err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(content.Content)
}
