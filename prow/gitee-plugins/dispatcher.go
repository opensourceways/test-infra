package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

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

	// ec is an http client used for dispatching events
	// to external plugin services.
	ec http.Client
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

func (d *dispatcher) Dispatch(eventType, eventGUID string, payload []byte, h http.Header) error {
	l := logrus.WithFields(
		logrus.Fields{
			"event-type":     eventType,
			github.EventGUID: eventGUID,
		},
	)

	var srcRepo string
	switch eventType {
	case "Note Hook":
		var e gitee.NoteEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return err
		}
		if err := checkRepository(payload, e.Repository); err != nil {
			return err
		}
		srcRepo = e.Repository.FullName
		d.wg.Add(1)
		go d.handleNoteEvent(&e, l)

	case "Issue Hook":
		var ie gitee.IssueEvent
		if err := json.Unmarshal(payload, &ie); err != nil {
			return err
		}
		if err := checkRepository(payload, ie.Repository); err != nil {
			return err
		}
		srcRepo = ie.Repository.FullName
		d.wg.Add(1)
		go d.handleIssueEvent(&ie, l)

	case "Merge Request Hook":
		var pr gitee.PullRequestEvent
		if err := json.Unmarshal(payload, &pr); err != nil {
			return err
		}
		if err := checkRepository(payload, pr.Repository); err != nil {
			return err
		}
		srcRepo = pr.Repository.FullName
		d.wg.Add(1)
		go d.handlePullRequestEvent(&pr, l)

	case "Push Hook":
		var pe gitee.PushEvent
		if err := json.Unmarshal(payload, &pe); err != nil {
			return err
		}
		if err := checkRepository(payload, pe.Repository); err != nil {
			return err
		}
		srcRepo = pe.Repository.FullName
		d.wg.Add(1)
		go d.handlePushEvent(&pe, l)

	default:
		l.Debug("Ignoring unhandled event type")
	}
	//dispatcher hook event only to external plugins that require this event
	if eps := d.needDispatchExternalPlugins(eventType, srcRepo); len(eps) > 0 {
		go d.dispatchExternal(l, eps, payload, h)
	}
	return nil
}

func (d *dispatcher) needDispatchExternalPlugins(eventType, srcRepo string) []ExternalPlugin {
	var matching []ExternalPlugin
	srcOrg := strings.Split(srcRepo, "/")[0]
	for repo, ep := range d.c.Config().ExternalPlugins {
		if repo != srcRepo && repo != srcOrg {
			continue
		}
		for _, p := range ep {
			if len(p.Events) == 0 {
				matching = append(matching, p)
			} else {
				for _, et := range p.Events {
					if et != eventType {
						continue
					}
					matching = append(matching, p)
					break
				}
			}
		}
	}
	return matching
}

func (d *dispatcher) dispatchExternal(l *logrus.Entry, externalPlugins []ExternalPlugin, payload []byte, h http.Header) {
	h.Set("User-Agent", "ProwHook")
	for _, p := range externalPlugins {
		d.wg.Add(1)
		go func(p ExternalPlugin) {
			defer d.wg.Done()
			if err := d.dispatch(p.Endpoint, payload, h); err != nil {
				l.WithError(err).WithField("external-plugin", p.Name).Error("Error dispatching event to external plugin.")
			} else {
				l.WithField("external-plugin", p.Name).Info("Dispatched event to external plugin")
			}
		}(p)
	}
}

// dispatch creates a new request using the provided payload and headers
// and dispatches the request to the provided endpoint.
func (d *dispatcher) dispatch(endpoint string, payload []byte, h http.Header) error {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header = h
	resp, err := d.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("response has status %q and body %q", resp.Status, string(rb))
	}
	return nil
}

func (d *dispatcher) do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	backoff := 100 * time.Millisecond
	maxRetries := 5

	for retries := 0; retries < maxRetries; retries++ {
		resp, err = d.ec.Do(req)
		if err == nil {
			break
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	return resp, err
}

func (d *dispatcher) handlePullRequestEvent(pr *gitee.PullRequestEvent, l *logrus.Entry) {
	defer d.wg.Done()

	l = l.WithFields(logrus.Fields{
		github.OrgLogField:  pr.Repository.Namespace,
		github.RepoLogField: pr.Repository.Path,
		github.PrLogField:   pr.PullRequest.Number,
		"author":            pr.PullRequest.Head.User.Login,
		"url":               pr.PullRequest.HtmlUrl,
	})
	l.Infof("Pull request %s.", *pr.Action)

	for p, h := range d.pullRequestHandlers(pr.Repository.Namespace, pr.Repository.Path) {
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
		github.OrgLogField:  i.Repository.Namespace,
		github.RepoLogField: i.Repository.Path,
		github.PrLogField:   i.Issue.Number,
		"author":            i.Issue.User.Login,
		"url":               i.Issue.HtmlUrl,
	})
	l.Infof("Issue %s.", *i.Action)

	for p, h := range d.issueHandlers(i.Repository.Namespace, i.Repository.Path) {
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
		github.OrgLogField:  pe.Repository.Namespace,
		github.RepoLogField: pe.Repository.Path,
		"ref":               pe.Ref,
		"head":              pe.After,
	})
	l.Info("Push event.")

	for p, h := range d.pushEventHandlers(pe.Repository.Owner.Name, pe.Repository.Path) {
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
		github.OrgLogField:  e.Repository.Namespace,
		github.RepoLogField: e.Repository.Path,
		github.PrLogField:   n,
		"review":            e.Comment.Id,
		"commenter":         e.Comment.User.Login,
		"url":               e.Comment.HtmlUrl,
	})
	l.Infof("Note %s.", *e.Action)

	for p, h := range d.noteEventHandlers(e.Repository.Namespace, e.Repository.Path) {
		d.wg.Add(1)

		go func(p string, h NoteEventHandler) {
			defer d.wg.Done()

			if err := h(e, l); err != nil {
				l.WithField("plugin", p).WithError(err).Error("Error handling NoteEvent.")
			}
		}(p, h)
	}
}

func checkRepository(payload []byte, rep *gitee.ProjectHook) error {
	if rep == nil {
		return fmt.Errorf("event repository is empty,payload: %s", string(payload))
	}
	return nil
}
