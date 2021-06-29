package assign

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
	prowConfig "k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pluginhelp"
	origina "k8s.io/test-infra/prow/plugins/assign"
)

var collaboratorRe = regexp.MustCompile(`(?mi)^/(add|rm)-collaborator(( @?[-\w]+?)*)\s*$`)

type assign struct {
	getPluginConfig plugins.GetPluginConfig
	gec             giteeClient
}

func NewAssign(f plugins.GetPluginConfig, gec giteeClient) plugins.Plugin {
	return &assign{
		getPluginConfig: f,
		gec:             gec,
	}
}

func (a *assign) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	ph, _ := origina.HelpProvider(nil, nil)
	ph.Commands = ph.Commands[:1]
	ph.AddCommand(pluginhelp.Command{
		Usage:       "/[add|rm]-collaborator [[@]<username>...]",
		Description: "Assigns collaborator(s) to the issue",
		Featured:    true,
		WhoCanUse:   "Anyone can use the command, but the target user(s) must be an org member, a repo collaborator, or should have previously commented on the issue.",
		Examples:    []string{"/add-collaborator", "/rm-collaborator", "/add-collaborator @spongebob", "/add-collaborator spongebob patrick"},
	})
	return ph, nil
}

func (a *assign) PluginName() string {
	return "assign"
}

func (a *assign) NewPluginConfig() plugins.PluginConfig {
	return nil
}

func (a *assign) RegisterEventHandler(p plugins.Plugins) {
	name := a.PluginName()
	p.RegisterNoteEventHandler(name, a.handleNoteEvent)
}

func (a *assign) handleNoteEvent(e *sdk.NoteEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handleNoteEvent")
	}()
	ew := gitee.NewNoteEventWrapper(e)
	if !ew.IsCreatingCommentEvent() {
		log.Debug("Event is not a creation of a comment, skipping.")
		return nil
	}

	var n int32
	isPR := true
	if ew.IsPullRequest() {
		n = ew.PullRequest.Number
	} else if ew.IsIssue() {
		isPR = false
		a.handleAppointCollaborator(gitee.NewIssueNoteEvent(e), log)
	} else {
		log.Debug("not supported note type")
		return nil
	}

	ce := github.GenericCommentEvent{
		Repo: github.Repo{
			Owner: github.User{Login: e.Repository.Namespace},
			Name:  e.Repository.Path,
		},
		Body:    e.Comment.Body,
		User:    github.User{Login: e.Comment.User.Login},
		Number:  int(n),
		HTMLURL: e.Comment.HtmlUrl,
		IsPR:    isPR,
	}

	var f func(mu github.MissingUsers) string
	if isPR {
		f = buildAssignPRFailureComment(a, ce.Repo.Owner.Login, ce.Repo.Name)
	} else {
		f = buildAssignIssueFailureComment(a, ce.Repo.Owner.Login, ce.Repo.Name)
	}
	return origina.HandleAssign(ce, &ghclient{giteeClient: a.gec, e: e}, f, log)
}

func (a *assign) handleAppointCollaborator(ew gitee.IssueNoteEvent, log *logrus.Entry) {
	matches := collaboratorRe.FindAllStringSubmatch(ew.GetComment(), -1)
	if len(matches) == 0 {
		return
	}

	toAdd, toRemove := parseActionCollaborators(ew.GetCommenter(), matches)
	org, repo := ew.GetOrgRep()
	number := ew.GetIssueNumber()
	needUpdates, missUser, err := a.filterCollaborators(org, repo, number, toAdd, toRemove)
	if err != nil {
		log.Error(err)
		return
	}
	if len(missUser) > 0 {
		comment := fmt.Sprintf(
			"@%s gitee didn't allow you to [add|remove] collaborators, please check this users: **%s** are legitimate(must be an org/repo member or already assigner).",
			ew.GetCommenter(),
			strings.Join(missUser, ","),
		)
		if err = a.gec.CreateIssueComment(org, repo, number, comment); err != nil {
			log.Error(err)
		}
		return
	}
	// adapter gitee api
	collaborator := "0"
	if len(needUpdates) > 0 {
		collaborator = strings.Join(needUpdates, ",")
	}
	param := sdk.IssueUpdateParam{
		Repo:          repo,
		Collaborators: collaborator,
	}

	if _, err = a.gec.UpdateIssue(org, number, param); err != nil {
		log.Error(err)
	}
}

func (a *assign) filterCollaborators(org, repo, number string, add, rm []string) (updates, miss []string, err error) {
	issue, err := a.gec.GetIssue(org, repo, number)
	if err != nil {
		return
	}
	repoCB, err := a.gec.ListCollaborators(org, repo)
	if err != nil {
		return
	}
	addSet := sets.NewString(add...)
	members := sets.NewString(getCollaborators(repoCB)...)
	missAdd := addSet.Difference(members)
	if issue.Assignee.Login != "" && addSet.Has(issue.Assignee.Login) {
		missAdd.Insert(issue.Assignee.Login)
	}
	if missAdd.Len() > 0 {
		miss = missAdd.List()
		return
	}
	ccs := sets.NewString()
	for _, v := range issue.Collaborators {
		ccs.Insert(v.Login)
	}
	//delete outdated collaborators and merge collaborators that need to be added and removed
	updates = ccs.Intersection(members).Insert(add...).Delete(rm...).List()
	return
}

func buildAssignPRFailureComment(a *assign, org, repo string) func(mu github.MissingUsers) string {

	return func(mu github.MissingUsers) string {
		v, err := a.gec.ListCollaborators(org, repo)
		if err == nil {
			v1 := getCollaborators(v)

			return fmt.Sprintf("Gitee didn't allow you to assign to: %s.\n\nChoose following members as assignees.\n- %s", strings.Join(mu.Users, ", "), strings.Join(v1, "\n- "))
		}

		return fmt.Sprintf("Gitee didn't allow you to assign to: %s.", strings.Join(mu.Users, ", "))
	}
}

func buildAssignIssueFailureComment(a *assign, org, repo string) func(mu github.MissingUsers) string {

	return func(mu github.MissingUsers) string {
		if len(mu.Users) > 1 {
			return "Can only assign one person to an issue."
		}

		v, err := a.gec.ListCollaborators(org, repo)
		if err == nil {
			v1 := getCollaborators(v)

			return fmt.Sprintf("Gitee didn't allow you to assign to: %s.\n\nChoose one of following members as assignee.\n- %s", mu.Users[0], strings.Join(v1, "\n- "))
		}

		return fmt.Sprintf("Gitee didn't allow you to assign to: %s.", mu.Users[0])
	}
}

func parseActionCollaborators(commenter string, matches [][]string) (toAdd, toRemove []string) {
	users := map[string]bool{}
	for _, re := range matches {
		add := re[1] == "add"
		if re[2] == "" {
			users[commenter] = add
		} else {
			for _, login := range origina.ParseLogins(re[2]) {
				users[login] = add
			}
		}
	}
	for login, add := range users {
		if add {
			toAdd = append(toAdd, login)
		} else {
			toRemove = append(toRemove, login)
		}
	}
	return
}

func getCollaborators(u []github.User) []string {
	r := make([]string, len(u))
	for i, item := range u {
		r[i] = item.Login
	}
	return r
}
