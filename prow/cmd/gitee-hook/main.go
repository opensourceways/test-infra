/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/prow/interrupts"

	"k8s.io/test-infra/pkg/flagutil"
	prowv1 "k8s.io/test-infra/prow/client/clientset/versioned/typed/prowjobs/v1"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/config/secret"
	prowflagutil "k8s.io/test-infra/prow/flagutil"
	"k8s.io/test-infra/prow/git/v2"
	"k8s.io/test-infra/prow/gitee"
	hook "k8s.io/test-infra/prow/gitee-hook"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	originh "k8s.io/test-infra/prow/hook"
	"k8s.io/test-infra/prow/logrusutil"
	"k8s.io/test-infra/prow/metrics"
	"k8s.io/test-infra/prow/pjutil"
	"k8s.io/test-infra/prow/repoowners"
)

type options struct {
	port int

	configPath    string
	jobConfigPath string
	pluginConfig  string

	dryRun      bool
	gracePeriod time.Duration
	kubernetes  prowflagutil.KubernetesOptions
	bugzilla    prowflagutil.BugzillaOptions
	gitee       prowflagutil.GiteeOptions

	webhookSecretFile string
	slackTokenFile    string
}

func (o *options) Validate() error {
	for _, group := range []flagutil.OptionGroup{&o.kubernetes, &o.gitee, &o.bugzilla} {
		if err := group.Validate(o.dryRun); err != nil {
			return err
		}
	}

	return nil
}

func gatherOptions(fs *flag.FlagSet, args ...string) options {
	var o options
	fs.IntVar(&o.port, "port", 8888, "Port to listen on.")

	fs.StringVar(&o.configPath, "config-path", "", "Path to config.yaml.")
	fs.StringVar(&o.jobConfigPath, "job-config-path", "", "Path to prow job configs.")
	fs.StringVar(&o.pluginConfig, "plugin-config", "/etc/plugins/plugins.yaml", "Path to plugin config file.")

	fs.BoolVar(&o.dryRun, "dry-run", true, "Dry run for testing. Uses API tokens but does not mutate.")
	fs.DurationVar(&o.gracePeriod, "grace-period", 180*time.Second, "On shutdown, try to handle remaining events for the specified duration. ")
	for _, group := range []flagutil.OptionGroup{&o.kubernetes, &o.bugzilla, &o.gitee} {
		group.AddFlags(fs)
	}

	fs.StringVar(&o.webhookSecretFile, "hmac-secret-file", "/etc/webhook/hmac", "Path to the file containing the GitHub HMAC secret.")
	fs.StringVar(&o.slackTokenFile, "slack-token-file", "", "Path to the file containing the Slack token to use.")
	fs.Parse(args)
	return o
}

func main() {
	logrusutil.ComponentInit()

	o := gatherOptions(flag.NewFlagSet(os.Args[0], flag.ExitOnError), os.Args[1:]...)
	if err := o.Validate(); err != nil {
		logrus.WithError(err).Fatal("Invalid options")
	}

	configAgent := &config.Agent{}
	if err := configAgent.Start(o.configPath, o.jobConfigPath); err != nil {
		logrus.WithError(err).Fatal("Error starting config agent.")
	}

	var tokens []string

	// Append the path of hmac and gitee secrets.
	tokens = append(tokens, o.webhookSecretFile)
	tokens = append(tokens, o.gitee.TokenPath)

	// This is necessary since slack token is optional.
	if o.slackTokenFile != "" {
		tokens = append(tokens, o.slackTokenFile)
	}

	if o.bugzilla.ApiKeyPath != "" {
		tokens = append(tokens, o.bugzilla.ApiKeyPath)
	}

	secretAgent := &secret.Agent{}
	if err := secretAgent.Start(tokens); err != nil {
		logrus.WithError(err).Fatal("Error starting secrets agent.")
	}

	pluginAgent := plugins.NewConfigAgent()
	if err := pluginAgent.Load(o.pluginConfig, false, nil); err != nil {
		logrus.WithError(err).Fatal("Error loading plugins config.")
	}

	cs, err := buildClients(&o, secretAgent, pluginAgent, configAgent.Config)
	if err != nil {
		logrus.WithError(err).Fatal("Error building clients.")
	}

	pm := plugins.NewPluginManager()

	initPlugins(configAgent.Config, pluginAgent, pm, cs)

	if err := pluginAgent.Start(o.pluginConfig, true, pm.HelpProviders()); err != nil {
		logrus.WithError(err).Fatal("Error loading plugins config.")
	}

	defer interrupts.WaitForGracefulShutdown()

	// Expose prometheus metrics
	metrics.ExposeMetrics("gitee-hook", configAgent.Config().PushGateway)
	pjutil.ServePProf()

	vf := func(w http.ResponseWriter, r *http.Request) (string, string, []byte, bool, int) {
		return gitee.ValidateWebhook(w, r, secretAgent.GetTokenGenerator(o.webhookSecretFile))
	}
	server := hook.NewServer(originh.NewMetrics(), vf, plugins.NewDispatcher(pluginAgent, pm))

	interrupts.OnInterrupt(func() {
		server.GracefulShutdown()

		if err := cs.giteeGitClient.Clean(); err != nil {
			logrus.WithError(err).Error("Could not clean up git client cache.")
		}
	})

	health := pjutil.NewHealth()

	// TODO remove this health endpoint when the migration to health endpoint is done
	// Return 200 on / for health checks.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})

	// For /hook, handle a webhook normally.
	http.Handle("/gitee-hook", server)
	// Serve plugin help information from /plugin-help.
	// TODO(zengchen1024) pluginhelp use the original plugins.Configuration
	//http.Handle("/gitee-plugin-help", pluginhelp.NewHelpAgent(pluginAgent, githubClient))

	httpServer := &http.Server{Addr: ":" + strconv.Itoa(o.port)}

	health.ServeReady()

	interrupts.ListenAndServe(httpServer, o.gracePeriod)
}

type clients struct {
	giteeClient    gitee.Client
	giteeGitClient git.ClientFactory
	ownersClient   repoowners.Interface
	prowJobClient  prowv1.ProwJobInterface
}

func buildClients(o *options, secretAgent *secret.Agent, pluginAgent *plugins.ConfigAgent, cfg config.Getter) (*clients, error) {
	giteeClient, err := o.gitee.GiteeClient(secretAgent, o.dryRun)
	if err != nil {
		return nil, err
	}
	giteeGitClient, err := o.gitee.GitClient(secretAgent, o.dryRun)
	if err != nil {
		return nil, err
	}

	prowJobClient, err := o.kubernetes.ProwJobClient(cfg().ProwJobNamespace, o.dryRun)
	if err != nil {
		return nil, fmt.Errorf("Error getting ProwJob client for infrastructure cluster: %w", err)
	}

	mdYAMLEnabled := func(org, repo string) bool {
		return pluginAgent.Config().MDYAMLEnabled(org, repo)
	}
	skipCollaborators := func(org, repo string) bool {
		return pluginAgent.Config().SkipCollaborators(org, repo)
	}
	ownersDirBlacklist := func() config.OwnersDirBlacklist {
		return cfg().OwnersDirBlacklist
	}
	ownersClient := repoowners.NewClient(giteeGitClient, giteeClient, mdYAMLEnabled, skipCollaborators, ownersDirBlacklist)

	cs := &clients{
		giteeClient:    giteeClient,
		giteeGitClient: giteeGitClient,
		ownersClient:   ownersClient,
		prowJobClient:  prowJobClient,
	}
	return cs, nil
}
