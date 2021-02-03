package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	"k8s.io/test-infra/prow/pluginhelp"
)


type server struct {
	tokenGenerator func() []byte
	config         func() hookAgentConfig
	log            *logrus.Entry
	wg             sync.WaitGroup
}

//GracefulShutdown Handle remaining requests
func (s *server) GracefulShutdown() {
	s.wg.Wait()
}

func helpProvider(_ []config.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: "The hookAgent plugin is used to distribute webhook events to third-party scripts.",
	}

	return pluginHelp, nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventType, eventGUID, payload, ok, _ := gitee.ValidateWebhook(w, r, s.tokenGenerator)
	if !ok {
		return
	}
	if err := s.handleEvent(eventType, eventGUID, payload); err != nil {
		s.log.WithError(err)
	}
}

func (s *server) handleEvent(eventType, eventGUID string, payload []byte) error {
	fullName := ""
	switch eventType {
	case "Note Hook":
		var e sdk.NoteEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return err
		}
		fullName = e.Repository.FullName
	case "Merge Request Hook":
		var pr sdk.PullRequestEvent
		if err := json.Unmarshal(payload, &pr); err != nil {
			return err
		}
		fullName = pr.Repository.FullName
	default:
		s.log.Debug("Ignoring unhandled event type", eventType, eventGUID)
	}
	if fullName == "" {
		return fmt.Errorf("invalidate webhook")
	}
	s.wg.Add(1)
	go s.execScript(fullName, eventType, string(payload))
	return nil
}

func (s *server) execScript(fullName, eventType, payload string) {
	defer s.wg.Done()
	cfg := s.config()
	script := cfg.getNeedHandleScript(fullName)
	for _, v := range script {
		s.wg.Add(1)
		go func(c ScriptCfg) {
			defer s.wg.Done()
			param := make([]string, 0, 4)
			tmp := strings.Trim(c.Endpoint, " ")
			if tmp != "" {
				param = append(param, c.Endpoint)
			}
			if c.PPLName == "" {
				param = append(param, payload)
			} else {
				param = append(param, c.PPLName, payload)
			}
			if c.PPLType == "" {
				param = append(param, "-t", eventType)
			} else {
				param = append(param, c.PPLType, eventType)
			}
			cmd, err := execCmd(c.Process, param...)
			s.log.Debug(string(cmd))
			if err != nil {
				s.log.Error(err)
			}

		}(v)
	}

}

func execCmd(ep string, args ...string) ([]byte, error) {
	command := exec.Command(ep, args...)
	return command.Output()
}
