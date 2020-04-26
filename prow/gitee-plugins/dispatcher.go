package plugins

import (
	"encoding/json"
	"fmt"
	"sync"

	"gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	hook "k8s.io/test-infra/prow/gitee-hook"
	"k8s.io/test-infra/prow/github"
)

func NewDispatcher(c *ConfigAgent, ps Plugins) hook.Dispatcher {
	return &dispatcher{c: c, ps: ps.(*plugins)}
}

type dispatcher struct {
	c  *ConfigAgent
	ps *plugins

	// Tracks running handlers for graceful shutdown
	wg sync.WaitGroup
}

func (d *dispatcher) issueHandlers(owner, repo string) map[string]IssueHandler {
	ps := d.getPlugins(owner, repo)
	hs := d.ps.issueHandlers

	r := map[string]IssueHandler{}
	for _, p := range ps {
		if h, ok := hs[p]; ok {
			r[p] = h
		}
	}
	return r
}

func (d *dispatcher) pullRequestHandlers(owner, repo string) map[string]PullRequestHandler {
	ps := d.getPlugins(owner, repo)
	hs := d.ps.pullRequestHandlers

	r := map[string]PullRequestHandler{}
	for _, p := range ps {
		if h, ok := hs[p]; ok {
			r[p] = h
		}
	}
	return r
}

func (d *dispatcher) pushEventHandlers(owner, repo string) map[string]PushEventHandler {
	ps := d.getPlugins(owner, repo)
	hs := d.ps.pushEventHandlers

	r := map[string]PushEventHandler{}
	for _, p := range ps {
		if h, ok := hs[p]; ok {
			r[p] = h
		}
	}
	return r
}

func (d *dispatcher) noteEventHandlers(owner, repo string) map[string]NoteEventHandler {
	ps := d.getPlugins(owner, repo)
	hs := d.ps.noteEventHandlers

	r := map[string]NoteEventHandler{}
	for _, p := range ps {
		if h, ok := hs[p]; ok {
			r[p] = h
		}
	}
	return r
}

func (d *dispatcher) getPlugins(owner, repo string) []string {
	var plugins []string

	c := d.c.Config()
	fullName := fmt.Sprintf("%s/%s", owner, repo)
	plugins = append(plugins, c.Plugins[owner]...)
	plugins = append(plugins, c.Plugins[fullName]...)

	return plugins
}

func (d *dispatcher) Wait() {
	d.wg.Wait() // Handle remaining requests
}

func (d *dispatcher) Dispatch(eventType, eventGUID string, payload []byte) error {
	l := logrus.WithFields(
		logrus.Fields{
			"event-type":     eventType,
			github.EventGUID: eventGUID,
		},
	)

	switch eventType {
	case "Note Hook":
		var e gitee.NoteEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return err
		}
		d.wg.Add(1)
		go d.handleNoteEvent(&e, l)

	case "Issue Hook":
		var ie gitee.IssueEvent
		if err := json.Unmarshal(payload, &ie); err != nil {
			return err
		}
		d.wg.Add(1)
		go d.handleIssueEvent(&ie, l)

	case "Merge Request Hook":
		var pr gitee.PullRequestEvent
		if err := json.Unmarshal(payload, &pr); err != nil {
			return err
		}
		d.wg.Add(1)
		go d.handlePullRequestEvent(&pr, l)

	case "Push Hook":
		var pe gitee.PushEvent
		if err := json.Unmarshal(payload, &pe); err != nil {
			return err
		}
		d.wg.Add(1)
		go d.handlePushEvent(&pe, l)

	default:
		l.Debug("Ignoring unhandled event type")
	}
	return nil
}

func (d *dispatcher) handlePullRequestEvent(pr *gitee.PullRequestEvent, l *logrus.Entry) {
	defer d.wg.Done()

	l = l.WithFields(logrus.Fields{
		github.OrgLogField:  pr.Repository.Owner.Login,
		github.RepoLogField: pr.Repository.Name,
		github.PrLogField:   pr.PullRequest.Number,
		"author":            pr.PullRequest.User.Login,
		"url":               pr.PullRequest.HtmlUrl,
	})
	l.Infof("Pull request %s.", *pr.Action)

	for p, h := range d.pullRequestHandlers(pr.Repository.Owner.Login, pr.Repository.Name) {
		d.wg.Add(1)

		go func(p string, h PullRequestHandler) {
			defer d.wg.Done()

			if err := h(pr, l); err != nil {
				l.WithField("plugin", p).WithError(err).Error("Error handling PullRequestEvent.")
			}
		}(p, h)
	}
}

func (d *dispatcher) handleIssueEvent(i *gitee.IssueEvent, l *logrus.Entry) {
	defer d.wg.Done()

	l = l.WithFields(logrus.Fields{
		github.OrgLogField:  i.Repository.Owner.Login,
		github.RepoLogField: i.Repository.Name,
		github.PrLogField:   i.Issue.Number,
		"author":            i.Issue.User.Login,
		"url":               i.Issue.HtmlUrl,
	})
	l.Infof("Issue %s.", *i.Action)

	for p, h := range d.issueHandlers(i.Repository.Owner.Login, i.Repository.Name) {
		d.wg.Add(1)

		go func(p string, h IssueHandler) {
			defer d.wg.Done()

			if err := h(i, l); err != nil {
				l.WithField("plugin", p).WithError(err).Error("Error handling IssueEvent.")
			}
		}(p, h)
	}
}

func (d *dispatcher) handlePushEvent(pe *gitee.PushEvent, l *logrus.Entry) {
	defer d.wg.Done()

	l = l.WithFields(logrus.Fields{
		github.OrgLogField:  pe.Repository.Owner.Login,
		github.RepoLogField: pe.Repository.Name,
		"ref":               pe.Ref,
		"head":              pe.After,
	})
	l.Info("Push event.")

	for p, h := range d.pushEventHandlers(pe.Repository.Owner.Name, pe.Repository.Name) {
		d.wg.Add(1)

		go func(p string, h PushEventHandler) {
			defer d.wg.Done()

			if err := h(pe, l); err != nil {
				l.WithField("plugin", p).WithError(err).Error("Error handling PushEvent.")
			}
		}(p, h)
	}
}

func (d *dispatcher) handleNoteEvent(e *gitee.NoteEvent, l *logrus.Entry) {
	defer d.wg.Done()

	var n interface{}
	switch *(e.NoteableType) {
	case "PullRequest":
		n = e.PullRequest.Number
	case "Issue":
		n = e.Issue.Number
	}

	l = l.WithFields(logrus.Fields{
		github.OrgLogField:  e.Repository.Owner.Login,
		github.RepoLogField: e.Repository.Name,
		github.PrLogField:   n,
		"review":            e.Comment.Id,
		"commenter":         e.Comment.User.Login,
		"url":               e.Comment.HtmlUrl,
	})
	l.Infof("Note %s.", *e.Action)

	for p, h := range d.noteEventHandlers(e.Repository.Owner.Login, e.Repository.Name) {
		d.wg.Add(1)

		go func(p string, h NoteEventHandler) {
			defer d.wg.Done()

			if err := h(e, l); err != nil {
				l.WithField("plugin", p).WithError(err).Error("Error handling NoteEvent.")
			}
		}(p, h)
	}
}
