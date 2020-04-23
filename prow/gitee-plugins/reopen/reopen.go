package reopen

import (
	"fmt"
	"regexp"
	"strings"

	"gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	giteeclient "k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/repoowners"
)

const (
	reopenIssueMessage = `this issue is reopened by: ***@%s***.`
	reopenCommand      = "REOPEN"
	pluginName         = "reopen"
)

type GetPluginConfig func(string) plugins.PluginConfig

type reopen struct {
	getPluginConfig GetPluginConfig
	ghc             giteeclient.Client
	oc              ownersClient
}

type ownersClient interface {
	LoadRepoOwners(org, repo, base string) (repoowners.RepoOwner, error)
}

func NewReopen(f GetPluginConfig, ghc giteeclient.Client, oc ownersClient) *reopen {
	return &reopen{
		getPluginConfig: f,
		ghc:             ghc,
		oc:              oc,
	}
}

func (a *reopen) GetPluginName() string {
	return pluginName
}

func (a *reopen) HandleNoteEvent(event *gitee.NoteEvent, log *logrus.Entry) error {
	if !isReopenCommand(event.Comment.Body) {
		return nil
	}
	switch *event.NoteableType {
	case "PullRequest":
		//
	case "Issue":
		if event.Issue.State != "closed" {
			log.Infof("It is return because event issue state is not closed.")
			return nil
		}
		// get basic informations
		comment := event.Comment.Body
		owner := event.Repository.Namespace
		repo := event.Repository.Name
		issueNumber := event.Issue.Number
		issueAuthor := event.Issue.User.Login
		commentAuthor := event.Comment.User.Login
		log.Infof("reopen started. comment: %s owner: %s repo: %s issueNumber: %s issueAuthor: %s commentAuthor: %s",
			comment, owner, repo, issueNumber, issueAuthor, commentAuthor)

		// check if current author has write permission
		localVarOptionals := &gitee.GetV5ReposOwnerRepoCollaboratorsUsernamePermissionOpts{}
		//localVarOptionals.AccessToken = nil

		// get permission
		permission, err := a.ghc.GetPermission(owner, repo, commentAuthor, localVarOptionals)
		if err != nil {
			log.Errorf("unable to get comment author permission: %v", err)
			return err
		}

		// permission: admin, write, read, none
		if permission.Permission == "admin" || permission.Permission == "write" || issueAuthor == commentAuthor {
			//  issue author or permission: admin, write
			body := gitee.IssueUpdateParam{}
			body.Repo = repo
			//body.AccessToken = nil
			body.State = "open"
			// build label string
			var strLabel string
			for _, l := range event.Issue.Labels {
				strLabel += l.Name + ","
			}
			strLabel = strings.TrimRight(strLabel, ",")
			if strLabel == "" {
				strLabel = ","
			}
			body.Labels = strLabel
			log.Infof("invoke api to reopen: %s", issueNumber)

			// patch state
			_, response, err := a.ghc.PatchIssuesNumber(owner, issueNumber, body)
			if err != nil {
				if response.StatusCode == 400 {
					log.Infof("reopen successfully with status code %d: %s", response.StatusCode, issueNumber)
				} else {
					log.Errorf("unable to reopen: %s err: %v", issueNumber, err)
					return err
				}
			} else {
				log.Infof("reopen successfully: %v", issueNumber)
			}

			// add comment
			bodyComment := gitee.IssueCommentPostParam{}
			//bodyComment.AccessToken = nil
			bodyComment.Body = fmt.Sprintf(reopenIssueMessage, commentAuthor)
			_, _, err = a.ghc.PostIssuesNumberComments(owner, repo, issueNumber, bodyComment)
			if err != nil {
				log.Errorf("unable to add comment in issue: %v", err)
			}
		}
	default:
		//
	}
	return nil
}

func isReopenCommand(c string) bool {
	commandRegex := regexp.MustCompile(`(?m)^/([^\s]+)[\t ]*([^\n\r]*)`)
	for _, match := range commandRegex.FindAllStringSubmatch(c, -1) {
		cmd := strings.ToUpper(match[1])
		if cmd == reopenCommand {
			return true
		}
	}
	return false
}
