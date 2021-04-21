package flagutil

import (
	"flag"
	"fmt"

	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/prow/config/secret"
	"k8s.io/test-infra/prow/git/v2"
	"k8s.io/test-infra/prow/gitee"
)

// GiteeOptions holds options for interacting with Gitee.
type GiteeOptions struct {
	TokenPath     string
	RepoCacheDir  string
	CacheRepoOnPV bool
}

// NewGiteeOptions creates a GiteeOptions with default values.
func NewGiteeOptions() *GiteeOptions {
	return &GiteeOptions{}
}

// AddFlags injects Gitee options into the given FlagSet.
func (o *GiteeOptions) AddFlags(fs *flag.FlagSet) {
	o.addFlags(true, fs)
}

// AddFlagsWithoutDefaultGiteeTokenPath injects Gitee options into the given
// Flagset without setting a default for for the giteeTokenPath, allowing to
// use an anonymous Gitee client
func (o *GiteeOptions) AddFlagsWithoutDefaultGiteeTokenPath(fs *flag.FlagSet) {
	o.addFlags(false, fs)
}

func (o *GiteeOptions) addFlags(wantDefaultGiteeTokenPath bool, fs *flag.FlagSet) {
	defaultGiteeTokenPath := ""
	if wantDefaultGiteeTokenPath {
		defaultGiteeTokenPath = "/etc/gitee/oauth"
	}
	fs.StringVar(&o.TokenPath, "gitee-token-path", defaultGiteeTokenPath, "Path to the file containing the Gitee OAuth secret.")
	fs.StringVar(&o.RepoCacheDir, "repo-cache-dir", "", "Path to which clone repo.")
	fs.BoolVar(&o.CacheRepoOnPV, "cache-repo-on-pv", false, "Path to which clone repo.")
}

// Validate validates Gitee options.
func (o *GiteeOptions) Validate(dryRun bool) error {
	if o.CacheRepoOnPV && o.RepoCacheDir == "" {
		return fmt.Errorf("must set repo-cache-dir if caching repo on persistent volume")
	}
	return nil
}

// GiteeClientWithLogFields returns a Gitee client with extra logging fields
func (o *GiteeOptions) GiteeClientWithLogFields(secretAgent *secret.Agent, dryRun bool, fields logrus.Fields) (gitee.Client, error) {
	generator, err := token(o.TokenPath, secretAgent)
	if err != nil {
		return nil, err
	}

	return gitee.NewClient(generator), nil
}

// GiteeClient returns a Gitee client.
func (o *GiteeOptions) GiteeClient(secretAgent *secret.Agent, dryRun bool) (client gitee.Client, err error) {
	return o.GiteeClientWithLogFields(secretAgent, dryRun, logrus.Fields{})
}

// GitClient returns a Git client factory.
func (o *GiteeOptions) GitClient(secretAgent *secret.Agent, dryRun bool) (git.ClientFactory, error) {
	f, err := token(o.TokenPath, secretAgent)
	if err != nil {
		return nil, err
	}

	c, err := o.GiteeClient(secretAgent, false)
	if err != nil {
		return nil, err
	}

	userInfo := func() (name, email string, err error) {
		u, err := c.BotUser()
		if err != nil {
			return "", "", err
		}
		return u.Name, u.Email, nil
	}

	setOpt := func(opts *git.ClientFactoryOpts) {
		opts.Host = "gitee.com"
		opts.Username = c.BotName
		opts.Token = f
		opts.GitUser = userInfo
		opts.Censor = secretAgent.Censor
		if o.RepoCacheDir != "" {
			opts.CacheDirBase = &o.RepoCacheDir
		}
	}
	if o.CacheRepoOnPV {
		return git.NewClientFactoryOnPV(setOpt)
	}
	return git.NewClientFactory(setOpt)

}

func token(tokenPath string, secretAgent *secret.Agent) (func() []byte, error) {
	if tokenPath == "" {
		logrus.Warn("empty -gitee-token-path, will use anonymous gitee client")

		return func() []byte {
			return []byte{}
		}, nil
	}

	if secretAgent == nil {
		return nil, fmt.Errorf("cannot store token from %q without a secret agent", tokenPath)
	}
	return secretAgent.GetTokenGenerator(tokenPath), nil
}
