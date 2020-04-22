package plugins

import (
	"gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/pluginhelp"
)

// HelpProvider defines the function type that construct a pluginhelp.PluginHelp for enabled
// plugins. It takes into account the plugins configuration and enabled repositories.
type HelpProvider func(enabledRepos []config.OrgRepo) (*pluginhelp.PluginHelp, error)

// IssueHandler defines the function contract for a gitee.IssueEvent handler.
type IssueHandler func(e *gitee.IssueEvent, log *logrus.Entry) error

// PullRequestHandler defines the function contract for a gitee.PullRequestEvent handler.
type PullRequestHandler func(e *gitee.PullRequestEvent, log *logrus.Entry) error

// PushEventHandler defines the function contract for a gitee.PushEvent handler.
type PushEventHandler func(e *gitee.PushEvent, log *logrus.Entry) error

// NoteEventHandler defines the function contract for a gitee.NoteEvent handler.
type NoteEventHandler func(e *gitee.NoteEvent, log *logrus.Entry) error

type Plugins interface {
	RegisterHelper(name string, fn HelpProvider)
	RegisterIssueHandler(name string, fn IssueHandler)
	RegisterPullRequestHandler(name string, fn PullRequestHandler)
	RegisterPushEventHandler(name string, fn PushEventHandler)
	RegisterNoteEventHandler(name string, fn NoteEventHandler)

	HelpProviders() map[string]HelpProvider
}

func NewPluginManager() Plugins {
	return &plugins{
		pluginHelp:          map[string]HelpProvider{},
		issueHandlers:       map[string]IssueHandler{},
		pullRequestHandlers: map[string]PullRequestHandler{},
		pushEventHandlers:   map[string]PushEventHandler{},
		noteEventHandlers:   map[string]NoteEventHandler{},
	}
}

type plugins struct {
	pluginHelp          map[string]HelpProvider
	issueHandlers       map[string]IssueHandler
	pullRequestHandlers map[string]PullRequestHandler
	pushEventHandlers   map[string]PushEventHandler
	noteEventHandlers   map[string]NoteEventHandler
}

// RegisterHelper registers a plugin's helper method.
func (p *plugins) RegisterHelper(name string, fn HelpProvider) {
	p.pluginHelp[name] = fn
}

// RegisterIssueHandler registers a plugin's gitee.IssueEvent handler.
func (p *plugins) RegisterIssueHandler(name string, fn IssueHandler) {
	p.issueHandlers[name] = fn
}

// RegisterPullRequestHandler registers a plugin's gitee.PullRequestEvent handler.
func (p *plugins) RegisterPullRequestHandler(name string, fn PullRequestHandler) {
	p.pullRequestHandlers[name] = fn
}

// RegisterPushEventHandler registers a plugin's gitee.PushEvent handler.
func (p *plugins) RegisterPushEventHandler(name string, fn PushEventHandler) {
	p.pushEventHandlers[name] = fn
}

// RegisterNoteEventHandler registers a plugin's gitee.NoteEvent handler.
func (p *plugins) RegisterNoteEventHandler(name string, fn NoteEventHandler) {
	p.noteEventHandlers[name] = fn
}

func (p *plugins) HelpProviders() map[string]HelpProvider {
	return p.pluginHelp
}
