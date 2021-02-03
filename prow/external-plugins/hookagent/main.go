package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/pkg/flagutil"
	"k8s.io/test-infra/prow/config/secret"
	prowflagutil "k8s.io/test-infra/prow/flagutil"
	"k8s.io/test-infra/prow/interrupts"
	"k8s.io/test-infra/prow/pluginhelp/externalplugins"
)

const (
	giteeJson = `{"user":"%s","access_token":"%s"}`
	fileName  = ".gitee_personal_token.json"
)

type options struct {
	port              int
	gitee             prowflagutil.GiteeOptions
	hookAgentConfig   string
	webhookSecretFile string
	botName           string
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
	logrus.SetLevel(logrus.DebugLevel)
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

	fileContent := fmt.Sprintf(giteeJson, o.botName, string(generator()))
	err = createGiteeTokenFile(fileContent)
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

func createGiteeTokenFile(content string) error {
	osType := runtime.GOOS
	dir := ""
	switch osType {
	case "linux":
		dir = "/root"
	case "windows":
		dir = "C:/Users/Administrator"
	default:
		return fmt.Errorf("The operating system is not supported ")
	}
	if !fileExist(dir) {
		return fmt.Errorf("%s not exists", dir)
	}
	if !isDir(dir) {
		return fmt.Errorf("%s not dir", dir)
	}
	path := filepath.Join(dir, fileName)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	_, err = writer.WriteString(content)
	if err != nil {
		return err
	}
	err = writer.Flush()
	if err != nil {
		return err
	}
	return nil
}

func fileExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func isDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}
