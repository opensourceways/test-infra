// Package size contains a Prow plugin which counts the number of lines changed
// in a pull request for gitee platform, buckets this number into a few size classes (S, L, XL, etc),
// and finally labels the pull request with this size.
package size

import (
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	prowConfig "k8s.io/test-infra/prow/config"
	giteeplugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/pluginhelp"
	"k8s.io/test-infra/prow/plugins"
	originsize "k8s.io/test-infra/prow/plugins/size"
)

type size struct {
	getPluginConfig giteeplugins.GetPluginConfig
	ghc             *clientAdapter
}

//NewSize create a size plugin
func NewSize(f giteeplugins.GetPluginConfig, client client) giteeplugins.Plugin {
	return &size{
		getPluginConfig: f,
		ghc:             &clientAdapter{client},
	}
}

func (s *size) HelpProvider(enabledRepos []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	c1 := plugins.Configuration{Size: s.config()}
	return originsize.HelpProvider(&c1, enabledRepos)
}

func (s *size) PluginName() string {
	return "size"
}

func (s *size) NewPluginConfig() giteeplugins.PluginConfig {
	return &configuration{}
}

func (s *size) RegisterEventHandler(p giteeplugins.Plugins) {
	p.RegisterPullRequestHandler(s.PluginName(), s.handlePullRequestEvent)
}

func (s *size) handlePullRequestEvent(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	funcStart := time.Now()
	defer func() {
		log.WithField("duration", time.Since(funcStart).String()).Debug("Completed handlePullRequest")
	}()

	if e.PullRequest.State != "open" {
		log.Debug("Pull request state is not open, skipping...")
		return nil
	}

	pe := giteeplugins.ConvertPullRequestEvent(e)
	cfg := s.config()

	return originsize.Handle(s.ghc, cfg, log, pe)
}

func (s *size) config() (cfg plugins.Size) {
	config := s.getPluginConfig(s.PluginName())
	if config == nil {
		return
	}

	if c, ok := config.(*configuration); ok {
		return c.Size
	}
	return

}
