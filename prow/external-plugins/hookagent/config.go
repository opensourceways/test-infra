package main

import (
	"io/ioutil"
	"strings"

	"sigs.k8s.io/yaml"
)

type hookAgentConfig struct {
	Scripts []ScriptCfg `json:"scripts"`
}

//ScriptCfg External plugin script configuration
type ScriptCfg struct {
	Name     string   `json:"name"`
	Process  string   `json:"process"`
	Endpoint string   `json:"endpoint"`
	Repos    []string `json:"repos"`
	PPLName  string   `json:"pplname"`
	PPLType  string   `json:"ppltype"`
}

func (hac *hookAgentConfig) getNeedHandleScript(fullName string) map[string]ScriptCfg {
	needs := make(map[string]ScriptCfg, 0)
	ns := strings.Split(fullName, "/")[0]
	for _, s := range hac.Scripts {
		//all hook event will dispatch to script when not config repos
		if len(s.Repos) == 0 {
			needs[s.Name] = s
			continue
		}
		for _, repo := range s.Repos {
			if repo != fullName && repo != ns {
				continue
			}
			needs[s.Name] = s
			break
		}
	}
	return needs
}

func load(path string) (hookAgentConfig, error) {
	c := hookAgentConfig{}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, nil
}
