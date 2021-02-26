package label


type labelCfg struct {
	// AdditionalLabels is a set of additional labels enabled for use
	// on top of the existing "kind/*", "priority/*", and "sig/*" labels.
	AdditionalLabels []string `json:"additional_labels"`
	//LimitLabels restrict PR authors from using labels added by gitee web pages
	LimitLabels []string `json:"limit_labels"`
	//ClearLabels Labels that need to be cleared when the source_branch_change event occurs in PR
	ClearLabels []string  `json:"clear_labels"`
}

type configuration struct {
	Label labelCfg `json:"label,omitempty"`
}

func (cfg *configuration) Validate() error {
	return nil
}

func (cfg *configuration) SetDefault()  {

}



