package close

import (
	"fmt"
	"regexp"
	"strings"

	"gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/prow/config"
	giteeclient "k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/pluginhelp"
	"k8s.io/test-infra/prow/repoowners"
)

const (
	closeIssueMessage       = `this issue is closed by: ***@%s***.`
	closePullRequestMessage = `this pull request is closed by: ***@%s***.`
	closeCommand            = "CLOSE"
	pluginName              = "close"
)

type ownersClient interface {
	LoadRepoOwners(org, repo, base string) (repoowners.RepoOwner, error)
}

type close struct {
	getPluginConfig plugins.GetPluginConfig
	ghc             giteeclient.Client
	oc              ownersClient
}

func NewClose(f plugins.GetPluginConfig, ghc giteeclient.Client, oc ownersClient) plugins.Plugin {
	return &close{
		getPluginConfig: f,
		ghc:             ghc,
		oc:              oc,
	}
}

func (a *close) HelpProvider(enabledRepos []config.OrgRepo) (*pluginhelp.PluginHelp, error) {
	configInfo := map[string]string{}
	for _, repo := range enabledRepos {
		configInfo[repo.String()] = fmt.Sprintf("The authorized GitHub organization for this repository is %q.", repo)
	}
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "close a pull request or an issue with the `/close` command in the plugin.",
		Config:      configInfo,
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/close",
		Description: "Close a pull request or an issue",
		Featured:    false,
		WhoCanUse:   "Anyone with permission",
		Examples:    []string{"/close"},
	})
	return pluginHelp, nil
}

func (a *close) NewPluginConfig() plugins.PluginConfig {
	return &configuration{}
}

func (a *close) RegisterEventHandler(p plugins.Plugins) {
	name := a.PluginName()
	p.RegisterNoteEventHandler(name, a.HandleNoteEvent)
}

func (a *close) PluginName() string {
	return pluginName
}

func (a *close) HandleNoteEvent(event *gitee.NoteEvent, glog *logrus.Entry) error {
	if !isCloseCommand(event.Comment.Body) {
		return nil
	}
	switch *event.NoteableType {
	case "PullRequest":
		// handle open
		if event.PullRequest.State == "open" {
			// get basic params
			comment := event.Comment.Body
			owner := event.Repository.Namespace
			repo := event.Repository.Name
			prAuthor := event.PullRequest.User.Login
			prNumber := event.PullRequest.Number
			commentAuthor := event.Comment.User.Login
			glog.Infof("close started. comment: %s prAuthor: %s commentAuthor: %s owner: %s repo: %s number: %d",
				comment, prAuthor, commentAuthor, owner, repo, prNumber)

			// check if current author has write permission
			localVarOptionals := &gitee.GetV5ReposOwnerRepoCollaboratorsUsernamePermissionOpts{}
			//localVarOptionals.AccessToken = nil
			// get permission
			permission, err := a.ghc.GetPermission(owner, repo, commentAuthor, localVarOptionals)
			if err != nil {
				glog.Errorf("unable to get comment author permission: %v", err)
				return err
			}
			// permission: admin, write, read, none
			if permission.Permission == "admin" || permission.Permission == "write" || prAuthor == commentAuthor {
				//  pr author or permission: admin, write
				body := gitee.PullRequestUpdateParam{}
				//body.AccessToken = nil
				body.State = "closed"
				glog.Infof("invoke api to close: %d", prNumber)

				// patch state
				_, response, err := a.ghc.UpdatePullRequestContext(owner, repo, prNumber, body)
				if err != nil {
					if response.StatusCode == 400 {
						glog.Infof("close successfully with status code %d: %d", response.StatusCode, prNumber)
					} else {
						glog.Errorf("unable to close: %d err: %v", prNumber, err)
						return err
					}
				} else {
					glog.Infof("close successfully: %v", prNumber)
				}
				// add comment
				err = a.ghc.CreatePRComment(owner, repo, int(prNumber), fmt.Sprintf(closePullRequestMessage, commentAuthor))
				if err != nil {
					glog.Errorf("unable to add comment in pullRequest: %v", err)
					return err
				}
				return nil
			}
		}
	case "Issue":
		// handle open
		if event.Issue.State == "open" {
			// get basic informations
			comment := event.Comment.Body
			owner := event.Repository.Namespace
			repo := event.Repository.Name
			issueNumber := event.Issue.Number
			issueAuthor := event.Issue.User.Login
			commentAuthor := event.Comment.User.Login
			glog.Infof("close started. comment: %s owner: %s repo: %s issueNumber: %s issueAuthor: %s commentAuthor: %s",
				comment, owner, repo, issueNumber, issueAuthor, commentAuthor)

			// check if current author has write permission
			localVarOptionals := &gitee.GetV5ReposOwnerRepoCollaboratorsUsernamePermissionOpts{}
			//localVarOptionals.AccessToken = nil
			// get permission
			permission, err := a.ghc.GetPermission(owner, repo, commentAuthor, localVarOptionals)
			if err != nil {
				glog.Errorf("unable to get comment author permission: %v", err)
				return err
			}
			// permission: admin, write, read, none
			if permission.Permission == "admin" || permission.Permission == "write" || issueAuthor == commentAuthor {
				//  issue author or permission: admin, write
				body := gitee.IssueUpdateParam{}
				body.Repo = repo
				//body.AccessToken = nil
				body.State = "closed"
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
				glog.Infof("invoke api to close: %s", issueNumber)

				// patch state
				_, response, err := a.ghc.PatchIssuesNumber(owner, issueNumber, body)
				if err != nil {
					if response.StatusCode == 400 {
						glog.Infof("close successfully with status code %d: %s", response.StatusCode, issueNumber)
					} else {
						glog.Errorf("unable to close: %s err: %v", issueNumber, err)
						return err
					}
				} else {
					glog.Infof("close successfully: %v", issueNumber)
				}
				// add comment
				bodyComment := gitee.IssueCommentPostParam{}
				//bodyComment.AccessToken = nil
				bodyComment.Body = fmt.Sprintf(closeIssueMessage, commentAuthor)
				_, _, err = a.ghc.PostIssuesNumberComments(owner, repo, issueNumber, bodyComment)
				if err != nil {
					glog.Errorf("unable to add comment in issue: %v", err)
					return err
				}
			}
		}
	default:
		//
	}
	return nil
}

func isCloseCommand(c string) bool {
	commandRegex := regexp.MustCompile(`(?m)^/([^\s]+)[\t ]*([^\n\r]*)`)
	for _, match := range commandRegex.FindAllStringSubmatch(c, -1) {
		cmd := strings.ToUpper(match[1])
		if cmd == closeCommand {
			return true
		}
	}
	return false
}
