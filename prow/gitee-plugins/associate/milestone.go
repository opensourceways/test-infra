package associate

import (
	"fmt"
	"regexp"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	log "github.com/sirupsen/logrus"
)

const (
	unsetMilestoneLabel   = "needs-milestone"
	unsetMilestoneComment = "@%s You have not selected a milestone,please select a milestone.After setting the milestone, you can use the **/check-milestone** command to remove the **needs-milestone** label."
)

var checkMilestoneRe = regexp.MustCompile(`(?mi)^/check-milestone\s*$`)

type milestoneClient interface {
	CreateIssueComment(org, repo string, number string, comment string) error
	RemoveIssueLabel(org, repo, number, label string) error
	AddIssueLabel(org, repo, number, label string) error
}

func handleIssueCreate(ghc milestoneClient, e *sdk.IssueEvent, log *log.Entry) error {
	if e.Milestone != nil && e.Milestone.Id != 0 {
		log.Debug(fmt.Sprintf("Milestones have been set when the issue (%s)was created", e.Issue.Number))
		return nil
	}
	owner := e.Repository.Namespace
	repo := e.Repository.Path
	number := e.Issue.Number
	author := e.Issue.User.Login
	return handleAddLabelAndComment(ghc, owner, repo, number, author)
}

func handleIssueNoteEvent(ghc milestoneClient, e *sdk.NoteEvent) error {
	// Only consider "/check-milestone" comments.
	if !checkMilestoneRe.MatchString(e.Comment.Body) {
		return nil
	}
	owner := e.Repository.Namespace
	repo := e.Repository.Path
	number := e.Issue.Number
	author := e.Issue.User.Login
	hasLabel := hasLabel(e.Issue.Labels, unsetMilestoneLabel)
	hasMile := e.Issue.Milestone != nil && e.Issue.Milestone.Id != 0
	if hasMile && hasLabel {
		return ghc.RemoveIssueLabel(owner, repo, number, unsetMilestoneLabel)
	}
	if !hasMile && !hasLabel {
		return handleAddLabelAndComment(ghc, owner, repo, number, author)
	}
	return nil
}

func handleAddLabelAndComment(ghc milestoneClient, owner, repo, number, author string) error {
	err := ghc.AddIssueLabel(owner, repo, number, unsetMilestoneLabel)
	if err != nil {
		return err
	}
	return ghc.CreateIssueComment(owner, repo, number, fmt.Sprintf(unsetMilestoneComment, author))
}
