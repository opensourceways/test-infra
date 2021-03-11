package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/git/v2"
	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pluginhelp"
	"k8s.io/test-infra/prow/plugins"
)

const pluginName = "syncpr"

var syncRe = regexp.MustCompile(`(?mi)^/sync\s*$`)

func helpProvider(enabledRepos []config.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The sync plugin synchronize pr to another host of git repository, such as gitee.",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/sync",
		Featured:    false,
		Description: "Synchronize pr to gitee.",
		WhoCanUse:   "Trusted members of the organization for the repo.",
		Examples:    []string{"/sync"},
	})
	return pluginHelp, nil
}

type githubClient interface {
	IsCollaborator(owner, repo, login string) (bool, error)
	GetPullRequest(org, repo string, number int) (*github.PullRequest, error)
	CreateComment(org, repo string, number int, comment string) error
}

type giteeClient interface {
	BotName() (string, error)
	CreatePullRequest(org, repo, title, body, head, base string, canModify bool) (sdk.PullRequest, error)
	UpdatePullRequest(org, repo string, number int32, param sdk.PullRequestUpdateParam) (sdk.PullRequest, error)
	GetPullRequests(org, repo string, opts gitee.ListPullRequestOpt) ([]sdk.PullRequest, error)
}

type server struct {
	tokenGenerator func() []byte
	config         func() syncPRConfig

	ghc  githubClient
	ghgc git.ClientFactory
	gec  giteeClient
	gegc git.ClientFactory

	log   *logrus.Entry
	robot string

	// Tracks running handlers for graceful shutdown
	wg sync.WaitGroup
}

func (s *server) GracefulShutdown() {
	s.wg.Wait() // Handle remaining requests
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventType, eventGUID, payload, ok, _ := github.ValidateWebhook(w, r, s.tokenGenerator)
	if !ok {
		return
	}
	fmt.Fprint(w, "Event received. Have a nice day.")

	if err := s.handleEvent(eventType, eventGUID, payload); err != nil {
		logrus.WithError(err).Error("Error parsing event.")
	}
}

func (s *server) handleEvent(eventType, eventGUID string, payload []byte) error {
	l := s.log.WithFields(
		logrus.Fields{
			"event-type":     eventType,
			github.EventGUID: eventGUID,
		},
	)

	switch eventType {
	case "issue_comment":
		var ic github.IssueCommentEvent
		if err := json.Unmarshal(payload, &ic); err != nil {
			return err
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			if err := s.handleIssueComment(l, ic); err != nil {
				l.WithError(err).Info("Synchronizing pr failed.")
			}
		}()
	default:
		logrus.Debugf("skipping event of type %q", eventType)
	}
	return nil
}

func (s *server) handleIssueComment(l *logrus.Entry, ic github.IssueCommentEvent) error {
	l = l.WithFields(logrus.Fields{
		github.OrgLogField:  ic.Repo.Owner.Login,
		github.RepoLogField: ic.Repo.Name,
		github.PrLogField:   ic.Issue.Number,
	})

	action := github.GenericCommentEventAction("")
	if ic.Action == github.IssueCommentActionCreated {
		action = github.GenericCommentActionCreated
	}

	e := &github.GenericCommentEvent{
		ID:           ic.Issue.ID,
		GUID:         ic.GUID,
		IsPR:         ic.Issue.IsPullRequest(),
		Action:       action,
		Body:         ic.Comment.Body,
		HTMLURL:      ic.Comment.HTMLURL,
		Number:       ic.Issue.Number,
		Repo:         ic.Repo,
		User:         ic.Comment.User,
		IssueAuthor:  ic.Issue.User,
		Assignees:    ic.Issue.Assignees,
		IssueState:   ic.Issue.State,
		IssueBody:    ic.Issue.Body,
		IssueHTMLURL: ic.Issue.HTMLURL,
	}

	return s.handle(l, e)
}

func (s *server) handle(log *logrus.Entry, e *github.GenericCommentEvent) error {
	// Only handle open PRs and new requests.
	if e.IssueState != "open" || !e.IsPR || e.Action != github.GenericCommentActionCreated {
		return nil
	}
	if !syncRe.MatchString(e.Body) {
		return nil
	}

	log.Info("Requested a pr synchronization.")

	org := e.Repo.Owner.Login
	repo := e.Repo.Name
	prNumber := e.Number

	// Check if can sync pr
	b, err := canSyncPR(s.ghc, e)
	if err != nil {
		return err
	}
	if !b {
		resp := "sync pull request is restricted to collaborators"
		response(s.ghc, resp, e)
		return nil
	}

	destOrg := s.config().syncPRFor(org, repo)
	if destOrg == "" {
		log.Warnf("can't find dest org for %s/%s", org, repo)
		return nil
	}

	// Get pr
	spr, err := s.ghc.GetPullRequest(org, repo, prNumber)
	if err != nil {
		return err
	}

	// Clone the repo, checkout the PR.
	r, err := s.ghgc.ClientFor(org, repo)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Clean(); err != nil {
			log.WithError(err).Error("Error cleaning up repo.")
		}
	}()

	startClone := time.Now()
	if err := r.CheckoutPullRequest(prNumber); err != nil {
		return err
	}
	log.WithField("duration", time.Since(startClone)).Info("Cloned and checked out PR.")

	branch := fmt.Sprintf("pull%d", prNumber)

	// Submit pr
	dpr, err := s.pushToGitee(org, repo, branch, r.Directory(), spr)
	if err != nil {
		return err
	}

	// Create comment's response
	desc, err := syncDesc(s.config().GithubCommentTemplate, dpr)
	if err != nil {
		return err
	}

	return response(s.ghc, desc, e)
}

func canSyncPR(ghc githubClient, e *github.GenericCommentEvent) (bool, error) {
	return ghc.IsCollaborator(e.Repo.Owner.Login, e.Repo.Name, e.User.Login)
}

func response(ghc githubClient, resp string, e *github.GenericCommentEvent) error {
	comment := plugins.FormatResponseRaw(e.Body, e.HTMLURL, e.User.Login, resp)
	return ghc.CreateComment(e.Repo.Owner.Login, e.Repo.Name, e.Number, comment)
}
