package tide

import (
	"fmt"

	"github.com/huaweicloud/golangsdk"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/test-infra/prow/github"
)

type configuration struct {
	Tide []pluginConfig `json:"tide,omitempty"`
}

func (c *configuration) Validate() error {
	if _, err := golangsdk.BuildRequestBody(c, ""); err != nil {
		return err
	}

	for i := range c.Tide {
		if err := c.Tide[i].validate(); err != nil {
			return err
		}
	}
	return nil
}

func (c *configuration) SetDefault() {
	for i := range c.Tide {
		c.Tide[i].setDefault()
	}
}

func (c *configuration) TideFor(org, repo string) *pluginConfig {
	fullName := fmt.Sprintf("%s/%s", org, repo)

	index := -1
	for i := range c.Tide {
		item := &(c.Tide[i])
		applyOrgRepo, applyOrg := item.canApply(org, fullName)

		if applyOrgRepo {
			if !applyOrg {
				return item

			}
			index = i
		}
	}

	if index >= 0 {
		return &(c.Tide[index])
	}

	return nil
}

type pluginConfig struct {
	// Repos is either of the form org/repos or just org.
	Repos []string `json:"repos" required:"true"`

	// ExcludedRepos is in the form org/repo.
	ExcludedRepos []string `json:"excluded_repos,omitempty"`

	// MergeMethod is the method to merge PR.
	// The default method of merge. Valid options are squash, rebase, and merge.
	MergeMethod github.PullRequestMergeType `json:"merge_method,omitempty"`

	// Labels specifies the ones which a PR must have to be merged.
	Labels []string `json:"labels" required:"true"`

	// MissingLabels specifies the ones which a PR must not have to be merged.
	MissingLabels []string `json:"missing_labels,omitempty"`
}

// The return value will be one of the following cases:
// true,  false: the config can be applied to the org/repo
// true,  true:  the config can be applied to the org and org/repo
// false, true:  the config can be applied to the org excepot org/repo
// false, false: the config can be applied to neither org or org/repo
func (p *pluginConfig) canApply(org, orgRepo string) (applyOrgRepo bool, applyOrg bool) {
	v := sets.NewString(p.Repos...)
	if v.Has(orgRepo) {
		applyOrgRepo = true
		return
	}

	if !v.Has(org) {
		return
	}

	applyOrg = true

	if len(p.ExcludedRepos) > 0 && sets.NewString(p.ExcludedRepos...).HasAny(orgRepo) {
		return
	}

	applyOrgRepo = true
	return
}

func (p *pluginConfig) setDefault() {
	if p.MergeMethod == "" {
		p.MergeMethod = github.MergeMerge
	}
}

func (p pluginConfig) validate() error {
	if p.MergeMethod != github.MergeMerge && p.MergeMethod != github.MergeSquash {
		return fmt.Errorf("unsupported merge method:%s", p.MergeMethod)
	}

	if sets.NewString(p.Repos...).HasAny(p.ExcludedRepos...) {
		return fmt.Errorf("some org or org/repo exists in both repos and excluded_repos")
	}

	return nil
}

func (p pluginConfig) labelMet(labels map[string]bool) bool {
	missing, exclude := p.labelDiff(labels)
	return len(missing) == 0 && len(exclude) == 0
}

func (p pluginConfig) labelDiff(labels map[string]bool) (missing []string, exclude []string) {
	v := sets.NewString()
	for k := range labels {
		v.Insert(k)
	}

	missing = sets.NewString(p.Labels...).Difference(v).List()

	if len(p.MissingLabels) > 0 {
		exclude = sets.NewString(p.MissingLabels...).Intersection(v).List()
	}
	return
}
