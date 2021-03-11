package main

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	sdk "gitee.com/openeuler/go-gitee/gitee"

	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/github"
)

func (s *server) pushToGitee(org, repo, localBranch, localDir string, spr *github.PullRequest) (*sdk.PullRequest, error) {
	// Push to dest repo
	robotRepo := s.config().middleRepo(org, repo)
	r, err := s.gegc.ClientFromDir(s.robot, robotRepo, localDir)
	if err != nil {
		return nil, err
	}

	if err := r.ForcePush(localBranch); err != nil {
		return nil, err
	}

	head := fmt.Sprintf("%s:%s", s.robot, localBranch)

	// Submit pr
	destOrg := s.config().syncPRFor(org, repo)
	return s.submitPR(spr, destOrg, repo, head)
}

func (s *server) submitPR(spr *github.PullRequest, destOrg, repo, head string) (*sdk.PullRequest, error) {
	desc, err := syncDesc(s.config().GiteeCommentTemplate, spr)
	if err != nil {
		return nil, err
	}

	body := fmt.Sprintf("%s\n\n%s", spr.Body, desc)

	dpr, err := s.queryPR(destOrg, repo, head)
	if err != nil {
		return nil, err
	}

	if dpr == nil {
		pr, err := s.gec.CreatePullRequest(destOrg, repo, spr.Title, body, head, "master", false)
		if err != nil {
			return nil, err
		}
		return &pr, nil
	}

	if strings.Compare(dpr.Body, body) != 0 {
		_, err = s.gec.UpdatePullRequest(destOrg, repo, dpr.Number, sdk.PullRequestUpdateParam{Body:body})
		if err != nil {
			return nil, err
		}
	}

	return dpr, nil
}

func (s *server) queryPR(org, repo, head string) (*sdk.PullRequest, error) {
	opt := gitee.ListPullRequestOpt{
		State: "open",
		Head:  head,
		Base:  "master",
	}

	prs, err := s.gec.GetPullRequests(org, repo, opt)
	if err != nil {
		return nil, err
	}

	switch len(prs) {
	case 0:
		return nil, nil
	case 1:
		return &prs[0], nil
	}

	return nil, fmt.Errorf("There are more than one prs in repo(%s/%s) which are open and created by %s", org, repo, opt.Head)
}

func syncDesc(templ *template.Template, data interface{}) (string, error) {
	var b bytes.Buffer
	err := templ.Execute(&b, data)

	if err != nil {
		return "", fmt.Errorf("error executing URL template: %v", err)
	}

	return b.String(), nil
}
