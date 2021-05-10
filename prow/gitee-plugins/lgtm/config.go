package lgtm

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	originp "k8s.io/test-infra/prow/plugins"
)

type configuration struct {
	Lgtm []pluginConfig `json:"lgtm,omitempty"`
}

func (c *configuration) Validate() error {
	return nil
}

func (c *configuration) SetDefault() {
}

func (c *configuration) LgtmFor(org, repo string) *pluginConfig {
	fullName := fmt.Sprintf("%s/%s", org, repo)
	for _, item := range c.Lgtm {
		if !sets.NewString(item.Repos...).Has(fullName) {
			continue
		}
		return &item
	}
	// If you don't find anything, loop again looking for an org config
	for _, item := range c.Lgtm {
		if !sets.NewString(item.Repos...).Has(org) {
			continue
		}
		return &item
	}
	return &pluginConfig{}
}

type pluginConfig struct {
	originp.Lgtm

	StrictReview bool `json:"strict_review,omitempty"`
	//MinReviewers at least number of reviewers
	MinReviewers int `json:"min_reviewers,omitempty"`
}
