package main

import (
	prowConfig "k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/gitee-plugins/approve"
	"k8s.io/test-infra/prow/gitee-plugins/assign"
	"k8s.io/test-infra/prow/gitee-plugins/associate"
	"k8s.io/test-infra/prow/gitee-plugins/checkpr"
	"k8s.io/test-infra/prow/gitee-plugins/cla"
	claeuler "k8s.io/test-infra/prow/gitee-plugins/cla-euler"
	"k8s.io/test-infra/prow/gitee-plugins/lgtm"
	"k8s.io/test-infra/prow/gitee-plugins/lifecycle"
	"k8s.io/test-infra/prow/gitee-plugins/slack"
	"k8s.io/test-infra/prow/gitee-plugins/trigger"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pluginhelp"
	originp "k8s.io/test-infra/prow/plugins"
)

func initPlugins(cfg prowConfig.Getter, agent *plugins.ConfigAgent, pm plugins.Plugins, cs *clients) error {
	gpc := func(name string) plugins.PluginConfig {
		return agent.Config().GetPluginConfig(name)
	}

	botname, err := cs.giteeClient.BotName()
	if err != nil {
		return err
	}

	var v []plugins.Plugin
	v = append(v, approve.NewApprove(gpc, cs.giteeClient, cs.ownersClient))
	v = append(v, assign.NewAssign(gpc, cs.giteeClient))
	v = append(v, lgtm.NewLGTM(gpc, agent.Config, cs.giteeClient, cs.ownersClient))
	v = append(v, trigger.NewTrigger(gpc, cfg, cs.giteeClient, cs.prowJobClient, cs.giteeGitClient))
	v = append(v, slack.NewSlack(gpc, botname))
	v = append(v, cla.NewCLA(gpc, cs.giteeClient))
	v = append(v, claeuler.NewCLA(gpc, cs.giteeClient))
	v = append(v, associate.NewAssociate(gpc, cs.giteeClient))
	v = append(v, checkpr.NewCheckPr(gpc, cs.giteeClient))
	v = append(v,lifecycle.NewLifeCycle(gpc,cs.giteeClient))

	for _, i := range v {
		name := i.PluginName()

		i.RegisterEventHandler(pm)
		pm.RegisterHelper(name, i.HelpProvider)

		agent.RegisterPluginConfigBuilder(name, i.NewPluginConfig)
	}

	return nil
}

func genHelpProvider(h plugins.HelpProvider) originp.HelpProvider {
	return func(_ *originp.Configuration, enabledRepos []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
		return h(enabledRepos)
	}
}

func resetPluginHelper(pm plugins.Plugins) {
	hs := pm.HelpProviders()
	ph := map[string]originp.HelpProvider{}
	for k, h := range hs {
		if h == nil {
			continue
		}
		ph[k] = genHelpProvider(h)
	}
	ph["cla"] = claHelpProvider
	originp.ResetPluginHelp(ph)
}

type pluginHelperAgent struct {
	agent *plugins.ConfigAgent
}

func (p pluginHelperAgent) Config() *originp.Configuration {
	c := p.agent.Config()
	return &originp.Configuration{
		Plugins: c.Plugins,
	}
}

type pluginHelperClient struct {
	c gitee.Client
}

func (p pluginHelperClient) GetRepos(org string, _ bool) ([]github.Repo, error) {
	repos, err := p.c.GetRepos(org)
	if err != nil {
		return nil, err
	}

	r := make([]github.Repo, 0, len(repos))
	for _, item := range repos {
		r = append(r, github.Repo{FullName: item.FullName})
	}
	return r, nil
}

func claHelpProvider(_ *originp.Configuration, enabledRepos []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The check-cla plugin rechecks the CLA status of a pull request. If the author of pull request has already signed CLA, the label `openlookeng-cla/yes` will be added, otherwise, the label of `openlookeng-cla/no` will be added.",
	}
	pluginHelp.AddCommand(pluginhelp.Command{
		Usage:       "/check-cla",
		Description: "Check the CLA status of PR",
		Featured:    true,
		WhoCanUse:   "Anyone can use the command.",
		Examples:    []string{"/check-cla"},
	})
	return pluginHelp, nil
}
