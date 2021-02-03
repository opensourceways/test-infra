package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"sigs.k8s.io/yaml"
)

type hookAgentConfig struct {
	HookAgent []ScriptCfg `json:"hook_agent,omitempty"`
}

//ScriptCfg External plugin script configuration
type ScriptCfg struct {
	//Name the name of the third-party script, used to determine whether the hook event is forwarded to itself
	Name string `json:"name"`
	//Process The executable program name or path of the script.
	Process string `json:"process"`
	//Endpoint The script program needs to execute the source code file,
	// if the script execution does not need to specify the source code file,
	// there is no need to set it.
	Endpoint string `json:"endpoint,omitempty"`
	//Repos The organization/repository of hook events to be processed by the script
	Repos []string `json:"repos,omitempty"`
	//PPLName pass the parameter name of the hook event payload, if it is empty, there will be no parameter name
	PPLName string `json:"pplname,omitempty"`
	//PPLType The type of the payload of the hook event, the default is -t
	PPLType string `json:"ppltype,omitempty"`
}

func (hac *hookAgentConfig) getNeedHandleScript(fullName string) map[string]ScriptCfg {
	needs := make(map[string]ScriptCfg, 0)
	ns := strings.Split(fullName, "/")[0]
	for _, s := range hac.HookAgent {
		//all hook event will dispatch to script when not config repos
		if len(s.Repos) == 0 {
			needs[s.Name] = s
			continue
		}
		for _, repo := range s.Repos {
			if repo == fullName || repo == ns {
				needs[s.Name] = s
				break
			}
		}
	}
	return needs
}

func (hac *hookAgentConfig) validate() error {
	var err error
	if len(hac.HookAgent) == 0 {
		return err
	}

	for _, v := range hac.HookAgent {
		if v.Name == "" {
			err = fmt.Errorf("config error: Each third-party script needs to set its own name ")
			break
		}
		if v.Process == "" {
			err = fmt.Errorf("config error: Each third-party script needs to set its own process ")
			break
		}
	}
	return err
}

func load(path string) (hookAgentConfig, error) {
	c := hookAgentConfig{}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err = yaml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	err = c.validate()
	return c, err
}
