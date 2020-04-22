package approve

import (
	originp "k8s.io/test-infra/prow/plugins"
)

type configuration struct {
	Approve []originp.Approve `json:"approve,omitempty"`
}

func (c *configuration) Validate() error {
	return nil
}

func (c *configuration) SetDefault() {
}
