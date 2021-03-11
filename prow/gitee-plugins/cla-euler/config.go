package claeuler

import (
	"fmt"

	"github.com/huaweicloud/golangsdk"
	"k8s.io/apimachinery/pkg/util/sets"
)

type configuration struct {
	CLA []pluginConfig `json:"cla,omitempty"`
}

func (c *configuration) Validate() error {
	_, err := golangsdk.BuildRequestBody(c, "")
	return err
}

func (c *configuration) SetDefault() {
}

func (c *configuration) CLAFor(org, repo string) *pluginConfig {
	fullName := fmt.Sprintf("%s/%s", org, repo)

	index := -1
	for i := range c.CLA {
		item := &(c.CLA[i])

		s := sets.NewString(item.Repos...)
		if s.Has(fullName) {
			return item
		}

		if s.Has(org) {
			index = i
		}
	}

	if index >= 0 {
		return &(c.CLA[index])
	}

	return nil
}

type pluginConfig struct {
	// Repos is either of the form org/repos or just org.
	Repos []string `json:"repos" required:"true"`

	// CLALabelYes is the cla label name for org/repos indicating
	// the cla has been signed
	CLALabelYes string `json:"cla_label_yes" required:"true"`

	// CLALabelNo is the cla label name for org/repos indicating
	// the cla has not been signed
	CLALabelNo string `json:"cla_label_no" required:"true"`

	// CheckURL is the url used to check whether the contributor has signed cla
	// The url has the format as https://**/{{org}}:{{repo}}?email={{email}}
	CheckURL string `json:"check_url" required:"true"`

	// SignURL is the url used to sign the cla
	SignURL string `json:"sign_url" required:"true"`
}