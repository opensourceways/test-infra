package main

import (
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/gitee-plugins/approve"
)

func initPlugins(agent *plugins.ConfigAgent, pm plugins.Plugins, cs *clients) {
	gpc := func(name string) plugins.PluginConfig {
		return agent.Config().GetPluginConfig(name)
	}

	a := approve.NewApprove(gpc, cs.giteeClient, cs.ownersClient)

	name := a.PluginName()
	a.RegisterEventHandler(pm)
	pm.RegisterHelper(name, a.HelpProvider)
	agent.RegisterPluginConfigBuilder(name, a.NewPluginConfig)
}
