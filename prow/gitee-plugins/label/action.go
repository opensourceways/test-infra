package label

import (
	sdk "gitee.com/openeuler/go-gitee/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
)

type noteEventAction interface {
	addLabel(label string) error
	addComment(comment string) error
	removeLabel(label string) error
	getAllLabels() ([]sdk.Label, error)
}

type IssueNoteAction struct {
	event  *sdk.NoteEvent
	client *label
}

func (ia *IssueNoteAction) addLabel(label string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(ia.event)
	if err != nil {
		return err
	}
	return ia.client.ghc.AddIssueLabel(org, repo, ia.event.Issue.Number, label)
}

func (ia *IssueNoteAction) addComment(comment string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(ia.event)
	if err != nil {
		return err
	}
	return ia.client.ghc.CreateGiteeIssueComment(org, repo, ia.event.Issue.Number, comment)
}

func (ia *IssueNoteAction) removeLabel(label string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(ia.event)
	if err != nil {
		return err
	}
	return ia.client.ghc.RemoveIssueLabel(org, repo, ia.event.Issue.Number, label)
}

func (ia *IssueNoteAction) getAllLabels() ([]sdk.Label, error) {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(ia.event)
	if err != nil {
		return nil, err
	}
	return ia.client.ghc.GetIssueLabels(org, repo, ia.event.Issue.Number)
}

type PRNoteAction struct {
	event  *sdk.NoteEvent
	client *label
}

func (pa *PRNoteAction) addLabel(label string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(pa.event)
	if err != nil {
		return err
	}
	return pa.client.ghc.AddPRLabel(org, repo, int(pa.event.PullRequest.Number), label)
}

func (pa *PRNoteAction) addComment(comment string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(pa.event)
	if err != nil {
		return err
	}
	return pa.client.ghc.CreatePRComment(org, repo, int(pa.event.PullRequest.Number), comment)
}

func (pa *PRNoteAction) removeLabel(label string) error {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(pa.event)
	if err != nil {
		return err
	}
	return pa.client.ghc.RemovePRLabel(org, repo, int(pa.event.PullRequest.Number), label)
}

func (pa *PRNoteAction) getAllLabels() ([]sdk.Label, error) {
	org, repo, err := plugins.GetOwnerAndRepoByEvent(pa.event)
	if err != nil {
		return nil, err
	}
	return pa.client.ghc.GetPRLabels(org, repo, int(pa.event.PullRequest.Number))
}
