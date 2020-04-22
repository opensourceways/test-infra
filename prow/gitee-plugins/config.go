/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugins

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/prow/labels"
	origin "k8s.io/test-infra/prow/plugins"
	"sigs.k8s.io/yaml"
)

type PluginConfig interface {
	Validate() error
	SetDefault()
}

// configuration is the top-level serialization target for plugin configuration.
type Configurations struct {
	// Plugins is a map of repositories (eg "k/k") to lists of
	// plugin names.
	// You can find a comprehensive list of the default avaulable plugins here
	// https://github.com/kubernetes/test-infra/tree/master/prow/plugins
	// note that you're also able to add external plugins.
	Plugins map[string][]string `json:"plugins,omitempty"`

	// Owners contains configuration related to handling OWNERS files.
	Owners origin.Owners `json:"owners,omitempty"`

	// Built-in plugins specific configuration.
	pluginConfigs map[string]PluginConfig
}

func (c *Configurations) GetPluginConfig(name string) PluginConfig {
	if pc, ok := c.pluginConfigs[name]; ok {
		return pc
	}
	return nil
}

// MDYAMLEnabled returns a boolean denoting if the passed repo supports YAML OWNERS config headers
// at the top of markdown (*.md) files. These function like OWNERS files but only apply to the file
// itself.
func (c *Configurations) MDYAMLEnabled(org, repo string) bool {
	full := fmt.Sprintf("%s/%s", org, repo)
	for _, elem := range c.Owners.MDYAMLRepos {
		if elem == org || elem == full {
			return true
		}
	}
	return false
}

// SkipCollaborators returns a boolean denoting if collaborator cross-checks are enabled for
// the passed repo. If it's true, approve and lgtm plugins rely solely on OWNERS files.
func (c *Configurations) SkipCollaborators(org, repo string) bool {
	full := fmt.Sprintf("%s/%s", org, repo)
	for _, elem := range c.Owners.SkipCollaborators {
		if elem == org || elem == full {
			return true
		}
	}
	return false
}

// EnabledReposForPlugin returns the orgs and repos that have enabled the passed plugin.
func (c *Configurations) EnabledReposForPlugin(plugin string) (orgs, repos []string) {
	for repo, plugins := range c.Plugins {
		found := false
		for _, candidate := range plugins {
			if candidate == plugin {
				found = true
				break
			}
		}
		if found {
			if strings.Contains(repo, "/") {
				repos = append(repos, repo)
			} else {
				orgs = append(orgs, repo)
			}
		}
	}
	return
}

func (c *Configurations) Validate() error {
	if len(c.Plugins) == 0 {
		logrus.Warn("no plugins specified-- check syntax?")
	}

	for _, p := range c.pluginConfigs {
		p.SetDefault()

		if err := p.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func load(path string, c *Configurations) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(b, c); err != nil {
		return err
	}

	for _, p := range c.pluginConfigs {
		if err := yaml.Unmarshal(b, p); err != nil {
			return err
		}
	}

	if c.Owners.LabelsBlackList == nil {
		c.Owners.LabelsBlackList = []string{labels.Approved, labels.LGTM}
	}

	if err := c.Validate(); err != nil {
		return err
	}

	return nil
}
