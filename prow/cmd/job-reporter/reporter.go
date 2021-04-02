package main

import (
	"fmt"

	"k8s.io/test-infra/prow/apis/prowjobs/v1"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/config/secret"
	"k8s.io/test-infra/prow/job-reporter/gitee"
	"k8s.io/test-infra/prow/job-reporter/github"
)

type reportClient interface {
	Report(pj *v1.ProwJob) ([]*v1.ProwJob, error)
	GetName() string
	ShouldReport(pj *v1.ProwJob) bool
}

func buildReporter(o *options, cfg config.Getter) (map[reportClient]int, error) {
	rs := map[reportClient]int{}

	var secretAgent secret.Agent
	if err := secretAgent.Start([]string{}); err != nil {
		return rs, fmt.Errorf("Error starting secret agent: %w", err)
	}

	githubReporter, err := newGithubReporter(o, &secretAgent, cfg)
	if err != nil {
		return rs, err
	}
	if githubReporter != nil {
		rs[githubReporter] = o.githubWorkers
	}

	giteeReporter, err := newGiteeReporter(o, &secretAgent, cfg)
	if err != nil {
		return rs, err
	}
	if giteeReporter != nil {
		rs[giteeReporter] = o.giteeWorkers
	}

	return rs, nil
}

func newGithubReporter(o *options, secretAgent *secret.Agent, cfg config.Getter) (reportClient, error) {
	if o.github.TokenPath == "" {
		return nil, nil
	}

	if o.githubWorkers <= 0 {
		return nil, nil
	}

	if err := secretAgent.Add(o.github.TokenPath); err != nil {
		return nil, fmt.Errorf("Error reading GitHub credentials: %w", err)
	}

	githubClient, err := o.github.GitHubClient(secretAgent, o.dryRun)
	if err != nil {
		return nil, fmt.Errorf("github client: %w", err)
	}

	return github.NewReporter(githubClient, cfg, v1.ProwJobAgent("")), nil
}

func newGiteeReporter(o *options, secretAgent *secret.Agent, cfg config.Getter) (reportClient, error) {
	if o.gitee.TokenPath == "" {
		return nil, nil
	}

	if o.giteeWorkers <= 0 {
		return nil, nil
	}

	if err := secretAgent.Add(o.gitee.TokenPath); err != nil {
		return nil, fmt.Errorf("Error reading Gitee credentials: %w", err)
	}

	giteeClient, err := o.gitee.GiteeClient(secretAgent, o.dryRun)
	if err != nil {
		return nil, fmt.Errorf("gitee client: %w", err)
	}

	return gitee.NewReporter(giteeClient, cfg, v1.ProwJobAgent(""))
}
