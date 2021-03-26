package label

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/sets"
)

type labelCfg struct {
	// Repos is either of the form org/repos or just org.
	Repos []string `json:"repos" required:"true"`
	// AdditionalLabels is a set of additional labels enabled for use
	// on top of the existing "kind/*", "priority/*", and "sig/*" labels.
	AdditionalLabels []string `json:"additional_labels"`
	//LimitLabels specifies labels which PR authors can't add through gitee web pages
	LimitLabels []string `json:"limit_labels"`
	//ClearLabels specifies labels that should be removed when the codes of PR are changed.
	ClearLabels []string `json:"clear_labels"`
}

type configuration struct {
	Label []labelCfg `json:"label,omitempty"`
}

func (cfg *configuration) Validate() error {
	return nil
}

func (cfg *configuration) SetDefault() {

}

func (cfg *configuration) LabelFor(org, repo string) *labelCfg {
	fullName := fmt.Sprintf("%s/%s", org, repo)

	index := -1
	for i := range cfg.Label {
		item := &(cfg.Label[i])

		s := sets.NewString(item.Repos...)
		if s.Has(fullName) {
			return item
		}

		if s.Has(org) {
			index = i
		}
	}

	if index >= 0 {
		return &(cfg.Label[index])
	}

	return nil
}
