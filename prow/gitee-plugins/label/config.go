package label

type labelCfg struct {
	// AdditionalLabels is a set of additional labels enabled for use
	// on top of the existing "kind/*", "priority/*", and "sig/*" labels.
	AdditionalLabels []string `json:"additional_labels"`
	//LimitLabels specifies labels which PR authors can't add through gitee web pages
	LimitLabels []string `json:"limit_labels"`
	//ClearLabels specifies labels that should be removed when the codes of PR are changed.
	ClearLabels []string `json:"clear_labels"`
}

type configuration struct {
	Label labelCfg `json:"label,omitempty"`
}

func (cfg *configuration) Validate() error {
	return nil
}

func (cfg *configuration) SetDefault() {

}
