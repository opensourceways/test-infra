package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
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
	gitee             prowflagutil.GiteeOptions
	hookAgentConfig   string
	webhookSecretFile string
	botName           string
	filePath          string
}

func (o *options) Validate() error {
	for _, group := range []flagutil.OptionGroup{&o.gitee} {
		if err := group.Validate(false); err != nil {
			return err
		}
	}
	return nil
}

func gatherOption() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.IntVar(&o.port, "port", 8888, "port to listen on.")
	fs.StringVar(&o.botName, "bot-name", "ci-bot", "the bot name")
	fs.StringVar(&o.filePath, "file-path", "/root/.gitee_personal_token.json", "The user token file path that the python script needs to use")
	fs.StringVar(&o.hookAgentConfig, "config", "/etc/plugins/config.yaml", "path to plugin config file.")
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
	logrus.SetLevel(logrus.InfoLevel)
	log := logrus.StandardLogger().WithField("plugin", "hookAgent")

	//config setting
	cfg, err := load(o.hookAgentConfig)
	if err != nil {
		log.WithError(err).Fatal("Error loading hookAgent config.")
	}
	secretAgent := &secret.Agent{}
	if err := secretAgent.Start([]string{o.gitee.TokenPath, o.webhookSecretFile}); err != nil {
		log.WithError(err).Fatal("Error starting secrets agent.")
	}
	generator := secretAgent.GetTokenGenerator(o.gitee.TokenPath)
	if len(generator()) == 0 {
		log.WithError(errors.New("token error")).Fatal()
	}

	err = createGiteeTokenFile(o.botName, string(generator()), o.filePath)
	if err != nil {
		log.WithError(err).Fatal("create token file fail")
	}
	//init server
	serv := &server{
		tokenGenerator: secretAgent.GetTokenGenerator(o.webhookSecretFile),
		config: func() hookAgentConfig {
			return cfg
		},
		log: log,
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

func createGiteeTokenFile(botName, token, path string) error {
	cj := struct {
		User        string `json:"user"`
		AccessToken string `json:"access_token"`
	}{
		botName,
		token,
	}
	con, err := json.Marshal(&cj)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, con, 0644)
}
