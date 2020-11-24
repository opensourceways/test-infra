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
	"fmt"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"

	"k8s.io/test-infra/prow/github"
	originh "k8s.io/test-infra/prow/hook"
)

type Server interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	GracefulShutdown()
}

type Dispatcher interface {
	Wait()
	Dispatch(eventType, eventGUID string, payload []byte, h http.Header) error
}

// ValidateWebhook ensures that the provided request conforms to the
// format of a webhook such as GitHub and the payload can be validated with
// the provided hmac secret. It returns the event type, the event guid,
// the payload of the request, whether the webhook is valid or not,
// and finally the resultant HTTP status code
type ValidateWebhook func(http.ResponseWriter, *http.Request) (string, string, []byte, bool, int)

// Server implements http.Handler. It validates incoming GitHub webhooks and
// then dispatches them to the appropriate plugins.
type server struct {
	vwh     ValidateWebhook
	metrics *originh.Metrics

	dispatcher Dispatcher
}

func NewServer(m *originh.Metrics, v ValidateWebhook, d Dispatcher) Server {
	return &server{
		dispatcher: d,
		vwh:        v,
		metrics:    m,
	}
}

// ServeHTTP validates an incoming webhook and puts it into the event channel.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventType, eventGUID, payload, ok, resp := s.vwh(w, r)
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
			"event-type":     eventType,
			github.EventGUID: eventGUID,
		},
	)
	// We don't want to fail the webhook due to a metrics error.
	if counter, err := s.metrics.WebhookCounter.GetMetricWithLabelValues(eventType); err != nil {
		l.WithError(err).Warn("Failed to get metric for eventType " + eventType)
	} else {
		counter.Inc()
	}

	return s.dispatcher.Dispatch(eventType, eventGUID, payload, h)
}

// GracefulShutdown implements a graceful shutdown protocol. It handles all requests sent before
// receiving the shutdown signal.
func (s *server) GracefulShutdown() {
	s.dispatcher.Wait()
}
