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
		WhoCanUse:   "Anyone can use the command, but the target user(s) must be the repo's member.",
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

	if !ew.IsIssue() && !ew.IsPullRequest() {
		log.Debug("not supported note type")
		return nil
	}

	if ew.IsIssue() {
		a.handleAppointCollaborator(gitee.NewIssueNoteEvent(e), log)
	}

	var n int32
	var f func(mu github.MissingUsers) string
	org, repo := ew.GetOrgRep()
	if ew.IsPullRequest() {
		f = buildAssignPRFailureComment(a, org, repo)
		n = ew.PullRequest.Number
	} else {
		f = buildAssignIssueFailureComment(a, org, repo)
	}

	ce := github.GenericCommentEvent{
		Repo: github.Repo{
			Owner: github.User{Login: org},
			Name:  repo,
		},
		Body:    ew.GetComment(),
		User:    github.User{Login: ew.GetCommenter()},
		Number:  int(n),
		HTMLURL: e.Comment.HtmlUrl,
		IsPR:    ew.IsPullRequest(),
	}

	return origina.HandleAssign(ce, &ghclient{giteeClient: a.gec, e: e}, f, log)
}

func (a *assign) handleAppointCollaborator(ew gitee.IssueNoteEvent, log *logrus.Entry) {
	matches := collaboratorRe.FindAllStringSubmatch(ew.GetComment(), -1)
	if len(matches) == 0 {
		return
	}

	commenter := ew.GetCommenter()
	var toAdd, toRemove []string
	save := func(login string, isAdd bool) {
		if isAdd {
			toAdd = append(toAdd, login)
		} else {
			toRemove = append(toRemove, login)
		}
	}

	for _, re := range matches {
		add := re[1] == "add"
		if re[2] == "" {
			save(commenter, add)
		} else {
			for _, login := range origina.ParseLogins(re[2]) {
				save(login, add)
			}
		}
	}

	org, repo := ew.GetOrgRep()
	number := ew.GetIssueNumber()
	result, miss, err := a.buildCollaborators(org, repo, number, toAdd, toRemove)
	if err != nil {
		log.Error(err)
		return
	}

	if len(miss) > 0 {
		comment := fmt.Sprintf("@%s:\n%s", commenter, strings.Join(miss, "\n"))
		if err = a.gec.CreateIssueComment(org, repo, number, comment); err != nil {
			log.Error(err)
		}
	}

	// for gitee api "0" means empty collaborator
	collaborator := "0"
	if len(result) > 0 {
		collaborator = strings.Join(result, ",")
	}
	param := sdk.IssueUpdateParam{
		Repo:          repo,
		Collaborators: collaborator,
	}

	if _, err = a.gec.UpdateIssue(org, number, param); err != nil {
		log.Error(err)
	}
}

func (a *assign) buildCollaborators(org, repo, number string, add, rm []string) (result, miss []string, err error) {
	issue, err := a.gec.GetIssue(org, repo, number)
	if err != nil {
		return
	}

	repoMembers := sets.NewString()
	if v, err1 := a.getCollaborators(org, repo); err1 == nil {
		repoMembers.Insert(v...)
	} else {
		err = err1
		return
	}

	toAdd := sets.NewString(add...)
	if v := issue.Assignee.Login; toAdd.Has(v) {
		miss = append(miss, fmt.Sprintf("The assignee( %s ) can not be collaborator at same time.", v))
		toAdd.Delete(v)
	}

	if v := toAdd.Difference(repoMembers); v.Len() > 0 {
		miss = append(miss, fmt.Sprintf("These persons( %s ) are not allowed to be added as collaborator which must be the member of repository.", strings.Join(v.List(), ", ")))
		toAdd = toAdd.Difference(v)
	}

	current := sets.NewString()
	for _, v := range issue.Collaborators {
		current.Insert(v.Login)
	}
	toRemove := sets.NewString(rm...)
	if v := toRemove.Difference(current); v.Len() > 0 {
		miss = append(miss, fmt.Sprintf("These persons( %s ) are not in the current collaborators and no need to be removed again.", strings.Join(v.List(), ", ")))
	}

	result = current.Intersection(repoMembers).Difference(toRemove).Union(toAdd).List()
	return
}

func (a *assign) getCollaborators(org, repo string) ([]string, error) {
	repoCB, err := a.gec.ListCollaborators(org, repo)
	if err != nil {
		return nil, err
	}
	r := make([]string, len(repoCB))
	for i, item := range repoCB {
		r[i] = item.Login
	}
	return r, nil
}

func buildAssignPRFailureComment(a *assign, org, repo string) func(mu github.MissingUsers) string {

	return func(mu github.MissingUsers) string {
		if v, err := a.getCollaborators(org, repo); err == nil {
			return fmt.Sprintf("Gitee didn't allow you to assign to: %s.\n\nChoose following members as assignees.\n- %s", strings.Join(mu.Users, ", "), strings.Join(v, "\n- "))
		}

		return fmt.Sprintf("Gitee didn't allow you to assign to: %s.", strings.Join(mu.Users, ", "))
	}
}

func buildAssignIssueFailureComment(a *assign, org, repo string) func(mu github.MissingUsers) string {

	return func(mu github.MissingUsers) string {
		if len(mu.Users) > 1 {
			return "Can only assign one person to an issue."
		}

		if v, err := a.getCollaborators(org, repo); err == nil {
			return fmt.Sprintf("Gitee didn't allow you to assign to: %s.\n\nChoose one of following members as assignee.\n- %s", mu.Users[0], strings.Join(v, "\n- "))
		}

		return fmt.Sprintf("Gitee didn't allow you to assign to: %s.", mu.Users[0])
	}
}
