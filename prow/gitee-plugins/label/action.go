package label

import (
	sdk "gitee.com/openeuler/go-gitee/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
)

type noteEventAction interface {
	addLabel(label []string) error
	addComment(comment string) error
	removeLabel(label string) error
	getAllLabels() ([]sdk.Label, error)
}

type IssueNoteAction struct {
	event  *sdk.NoteEvent
	client giteeClient
}

func (ia *IssueNoteAction) addLabel(label []string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(ia.event)
	if err != nil {
		return err
	}
	return ia.client.AddMultiIssueLabel(org, repo, ia.issueNumber(), label)
}

func (ia *IssueNoteAction) addComment(comment string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(ia.event)
	if err != nil {
		return err
	}
	return ia.client.CreateGiteeIssueComment(org, repo, ia.issueNumber(), comment)
}

func (ia *IssueNoteAction) removeLabel(label string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(ia.event)
	if err != nil {
		return err
	}
	return ia.client.RemoveIssueLabel(org, repo, ia.issueNumber(), label)
}

func (ia *IssueNoteAction) getAllLabels() ([]sdk.Label, error) {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(ia.event)
	if err != nil {
		return nil, err
	}
	return ia.client.GetIssueLabels(org, repo, ia.issueNumber())
}

func (ia *IssueNoteAction) issueNumber() string {
	return ia.event.Issue.Number
}

type PRNoteAction struct {
	event  *sdk.NoteEvent
	client giteeClient
}

func (pa *PRNoteAction) prNumber() int {
	return int(pa.event.PullRequest.Number)
}

func (pa *PRNoteAction) addLabel(label []string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(pa.event)
	if err != nil {
		return err
	}
	return pa.client.AddMultiPRLabel(org, repo, pa.prNumber(), label)
}

func (pa *PRNoteAction) addComment(comment string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(pa.event)
	if err != nil {
		return err
	}
	return pa.client.CreatePRComment(org, repo, pa.prNumber(), comment)
}

func (pa *PRNoteAction) removeLabel(label string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(pa.event)
	if err != nil {
		return err
	}
	return pa.client.RemovePRLabel(org, repo, pa.prNumber(), label)
}

func (pa *PRNoteAction) getAllLabels() ([]sdk.Label, error) {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(pa.event)
	if err != nil {
		return nil, err
	}
	return pa.client.GetPRLabels(org, repo, pa.prNumber())
}
