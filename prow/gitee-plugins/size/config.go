package size

import "k8s.io/test-infra/prow/plugins"

type configuration struct {
	Size plugins.Size `json:"size,omitempty"`
}

func (c *configuration) Validate() error {
	return nil
}

func (c *configuration) SetDefault() {
}
