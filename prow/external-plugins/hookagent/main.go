package main

import (
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/pkg/flagutil"
	"k8s.io/test-infra/prow/config/secret"
	prowflagutil "k8s.io/test-infra/prow/flagutil"
	"k8s.io/test-infra/prow/interrupts"
	"k8s.io/test-infra/prow/pluginhelp/externalplugins"
)

type options struct {
	port              int
	dryRun            bool
	gitee             prowflagutil.GiteeOptions
	hookAgentConfig   string
	webhookSecretFile string
}

func (o *options) Validate() error {
	for _, group := range []flagutil.OptionGroup{&o.gitee} {
		if err := group.Validate(o.dryRun); err != nil {
			return err
		}
	}
	return nil
}

func gatherOption() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.IntVar(&o.port, "port", 8888, "port to listen on.")
	fs.StringVar(&o.hookAgentConfig, "hookAgent-config", "/etc/plugins/plugins.yaml", "path to plugin config file.")
	fs.BoolVar(&o.dryRun, "dry-run", true, "dry run for testing. Uses API tokens but does not mutate.")
	fs.StringVar(&o.webhookSecretFile, "hmac-secret-file", "/etc/webhook/hmac", "path to the file containing the gitee HMAC secret")
	for _, group := range []flagutil.OptionGroup{&o.gitee} {
		group.AddFlags(fs)
	}
	_ = fs.Parse(os.Args[1:])
	return o
}

func main() {
	o := gatherOption()
	if err := o.Validate(); err != nil {
		logrus.Fatalf("Invalid options: %v", err)
	}
	logrus.SetFormatter(&logrus.JSONFormatter{})
	//Use global option from the prow config.
	logrus.SetLevel(logrus.DebugLevel)
	log := logrus.StandardLogger().WithField("plugin", "hookAgent")

	//config setting
	cfg, err := load(o.hookAgentConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Error loading hookAgent config.")
	}
	secretAgent := &secret.Agent{}
	if err := secretAgent.Start([]string{o.gitee.TokenPath, o.webhookSecretFile}); err != nil {
		log.WithError(err).Fatal("Error starting secrets agent.")
	}

	giteeClient, err := o.gitee.GiteeClient(secretAgent, o.dryRun)
	if err != nil {
		logrus.WithError(err).Fatal("Error getting Gitee client.")
	}
	giteeGitClient, err := o.gitee.GitClient(secretAgent, o.dryRun)
	if err != nil {
		logrus.WithError(err).Fatal("Error getting Gitee Git client.")
	}

	name, err := giteeClient.BotName()
	if err != nil {
		logrus.WithError(err).Fatal("Error getting Gitee botname.")
	}
	//init server
	serv := &server{
		tokenGenerator: secretAgent.GetTokenGenerator(o.webhookSecretFile),
		config: func() hookAgentConfig {
			return cfg
		},
		gec:   giteeClient,
		gegc:  giteeGitClient,
		log:   log,
		robot: name,
	}
	mux := http.NewServeMux()
	mux.Handle("/", serv)
	externalplugins.ServeExternalPluginHelp(mux, log, helpProvider)
	httpServer := &http.Server{Addr: ":" + strconv.Itoa(o.port), Handler: mux}
	defer interrupts.WaitForGracefulShutdown()
	interrupts.OnInterrupt(func() {
		serv.GracefulShutdown()
	})

	interrupts.ListenAndServe(httpServer, 5*time.Second)
}
