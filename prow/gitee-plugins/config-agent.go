package plugins

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type PluginConfigBuilder func() PluginConfig

type ConfigAgent struct {
	mut sync.RWMutex
	c   *Configurations
	pcb map[string]PluginConfigBuilder
}

func NewConfigAgent() *ConfigAgent {
	return &ConfigAgent{pcb: map[string]PluginConfigBuilder{}}
}

func (ca *ConfigAgent) Load(path string, checkUnknownPlugins bool, knownPlugins map[string]HelpProvider) error {
	pcs := make(map[string]PluginConfig)
	for n, b := range ca.pcb {
		pcs[n] = b()
	}

	c := &Configurations{pluginConfigs: pcs}

	err := load(path, c)
	if err != nil {
		return err
	}

	if checkUnknownPlugins {
		var errors []error
		for _, ps := range c.Plugins {
			for _, p := range ps {
				if h, ok := knownPlugins[p]; !ok || (h == nil) {
					errors = append(errors, fmt.Errorf("unknown plugin: %s", p))
				}
			}
		}
		if len(errors) > 0 {
			return utilerrors.NewAggregate(errors)
		}
	}

	ca.mut.Lock()
	defer ca.mut.Unlock()
	ca.c = c

	return nil
}

func (ca *ConfigAgent) RegisterPluginConfigBuilder(name string, b PluginConfigBuilder) {
	ca.pcb[name] = b
}

func (ca *ConfigAgent) Config() *Configurations {
	ca.mut.Lock()
	defer ca.mut.Unlock()

	return ca.c
}

// Start starts polling path for plugin config. If the first attempt fails,
// then start returns the error. Future errors will halt updates but not stop.
// If checkUnknownPlugins is true, unrecognized plugin names will make config
// loading fail.
func (ca *ConfigAgent) Start(path string, checkUnknownPlugins bool, knownPlugins map[string]HelpProvider) error {
	if err := ca.Load(path, checkUnknownPlugins, knownPlugins); err != nil {
		return err
	}

	ticker := time.Tick(1 * time.Minute)
	go func() {
		for range ticker {
			if err := ca.Load(path, checkUnknownPlugins, knownPlugins); err != nil {
				logrus.WithField("path", path).WithError(err).Error("Error loading plugin config.")
			}
		}
	}()
	return nil
}
