package reminder

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/sets"
)

type configuration struct {
	Reminder []pluginConfig `json:"reminder,omitempty"`
}

func (c *configuration) Validate() error {
	return nil
}

func (c *configuration) SetDefault() {
}

func (c *configuration) ReminderFor(org, repo string) *pluginConfig {
	fullName := fmt.Sprintf("%s/%s", org, repo)

	index := -1
	for i := range c.Reminder {
		item := &(c.Reminder[i])

		s := sets.NewString(item.Repos...)
		if s.Has(fullName) {
			return item
		}

		if s.Has(org) {
			index = i
		}
	}
	if index >= 0 {
		return &(c.Reminder[index])
	}
	return nil
}

type pluginConfig struct {
	// Repos is either of the form org/repos or just org.
	Repos []string `json:"repos" required:"true"`

	// HELPLabel is the essential label name for issue tracking
	HELPLabel string `json:"help_label" required:"true"`
}
