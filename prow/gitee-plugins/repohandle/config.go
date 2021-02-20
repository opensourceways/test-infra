package repohandle

type configuration struct {
	RepoHandler pluginConfig `json:"repoHandle,omitempty"`
}

type pluginConfig struct {
	RepoFiles  []cfgFilePath `json:"repoFiles"`
	SigFiles   []cfgFilePath `json:"sigFiles"`
	OwnerFiles []cfgFilePath `json:"ownerFiles"`
}

//cfgFilePath configuration of the location of the repository, owner and sig group configuration files
type cfgFilePath struct {
	Owner string `json:"owner,omitempty"`
	Repo  string `json:"repo,omitempty"`
	Path  string `json:"path,omitempty"`
	Ref   string `json:"ref,omitempty"`
	//Hash is only used for caching and does not need to be configured in the configuration file.
	Hash string `json:"hash,omitempty"`
}

type sigCfg struct {
	Sigs []sig `json:"sigCache"`
}

type sig struct {
	Name         string   `json:"name"`
	Repositories []string `json:"repositories"`
}

type owner struct {
	Maintainers []string `json:"maintainers"`
}

type Repository struct {
	Name              *string  `json:"name"`
	Description       *string  `json:"description"`
	ProtectedBranches []string `json:"protected_branches" yaml:"protected_branches"`
	Commentable       *bool    `json:"commentable"`
	Type              *string  `json:"type"`
	AutoInit          bool     `json:"autoInit" yaml:"autoInit"`
	RenameFrom        *string  `json:"rename_from" yaml:"rename_from"`
	Managers          []string `json:"managers"`
	Developers        []string `json:"developers"`
	Viewers           []string `json:"viewers"`
	Reporters         []string `json:"reporters"`
}

// IsCommentable returns if contributors are able to comment
// to the repository.
// It will be true only if Commentable is explicitly sepecified as true
func (r Repository) IsCommentable() bool {
	return r.Commentable != nil && *r.Commentable
}

type Repos struct {
	Community    string       `json:"community"`
	Repositories []Repository `json:"repositories"`
}

func (rf cfgFilePath) equal(d cfgFilePath) bool {
	return rf.Owner == d.Owner && rf.Repo == d.Repo && rf.Path == d.Path && rf.Ref == d.Ref
}

func (c *configuration) Validate() error {
	return nil
}

func (c *configuration) SetDefault() {

}
