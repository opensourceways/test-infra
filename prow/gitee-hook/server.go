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

package hook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"

	pm "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/github"
	originh "k8s.io/test-infra/prow/hook"
)

type Server interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	GracefulShutdown()
}

// Server implements http.Handler. It validates incoming GitHub webhooks and
// then dispatches them to the appropriate plugins.
type server struct {
	plugins        plugins
	tokenGenerator func() []byte
	metrics        *originh.Metrics

	// Tracks running handlers for graceful shutdown
	wg sync.WaitGroup
}

func NewServer(c *pm.ConfigAgent, ps pm.Plugins, m *originh.Metrics, tg func() []byte) Server {
	return &server{
		plugins:        plugins{c: c, ps: ps},
		tokenGenerator: tg,
		metrics:        m,
	}
}

// ServeHTTP validates an incoming webhook and puts it into the event channel.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventType, eventGUID, payload, ok, resp := github.ValidateWebhook(w, r, s.tokenGenerator)

	if counter, err := s.metrics.ResponseCounter.GetMetricWithLabelValues(strconv.Itoa(resp)); err != nil {
		logrus.WithFields(logrus.Fields{
			"status-code": resp,
		}).WithError(err).Error("Failed to get metric for reporting webhook status code")
	} else {
		counter.Inc()
	}

	if !ok {
		return
	}
	fmt.Fprint(w, "Event received. Have a nice day.")

	if err := s.demuxEvent(eventType, eventGUID, payload, r.Header); err != nil {
		logrus.WithError(err).Error("Error parsing event.")
	}
}

func (s *server) demuxEvent(eventType, eventGUID string, payload []byte, h http.Header) error {
	l := logrus.WithFields(
		logrus.Fields{
			eventTypeField:   eventType,
			github.EventGUID: eventGUID,
		},
	)
	// We don't want to fail the webhook due to a metrics error.
	if counter, err := s.metrics.WebhookCounter.GetMetricWithLabelValues(eventType); err != nil {
		l.WithError(err).Warn("Failed to get metric for eventType " + eventType)
	} else {
		counter.Inc()
	}
	switch eventType {
	case "issues":
		var i github.IssueEvent
		if err := json.Unmarshal(payload, &i); err != nil {
			return err
		}
		i.GUID = eventGUID
		s.wg.Add(1)
		go s.handleIssueEvent(l, i)
	case "issue_comment":
		var ic github.IssueCommentEvent
		if err := json.Unmarshal(payload, &ic); err != nil {
			return err
		}
		ic.GUID = eventGUID
		s.wg.Add(1)
		go s.handleIssueCommentEvent(l, ic)
	case "pull_request":
		var pr github.PullRequestEvent
		if err := json.Unmarshal(payload, &pr); err != nil {
			return err
		}
		pr.GUID = eventGUID
		s.wg.Add(1)
		go s.handlePullRequestEvent(l, pr)
	case "pull_request_review":
		var re github.ReviewEvent
		if err := json.Unmarshal(payload, &re); err != nil {
			return err
		}
		re.GUID = eventGUID
		s.wg.Add(1)
		go s.handleReviewEvent(l, re)
	case "pull_request_review_comment":
		var rce github.ReviewCommentEvent
		if err := json.Unmarshal(payload, &rce); err != nil {
			return err
		}
		rce.GUID = eventGUID
		s.wg.Add(1)
		go s.handleReviewCommentEvent(l, rce)
	case "push":
		var pe github.PushEvent
		if err := json.Unmarshal(payload, &pe); err != nil {
			return err
		}
		pe.GUID = eventGUID
		s.wg.Add(1)
		go s.handlePushEvent(l, pe)
	case "status":
		var se github.StatusEvent
		if err := json.Unmarshal(payload, &se); err != nil {
			return err
		}
		se.GUID = eventGUID
		s.wg.Add(1)
		go s.handleStatusEvent(l, se)
	default:
		l.Debug("Ignoring unhandled event type. (Might still be handled by external plugins.)")
	}
	return nil
}

// GracefulShutdown implements a graceful shutdown protocol. It handles all requests sent before
// receiving the shutdown signal.
func (s *server) GracefulShutdown() {
	s.wg.Wait() // Handle remaining requests
	return
}
