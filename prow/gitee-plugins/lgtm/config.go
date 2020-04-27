package lgtm

import (
	originp "k8s.io/test-infra/prow/plugins"
)

type configuration struct {
	Lgtm []originp.Lgtm `json:"lgtm,omitempty"`
}

func (c *configuration) Validate() error {
	return nil
}

func (c *configuration) SetDefault() {
}

