/*
Copyright 2017 The Kubernetes Authors.

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

package gitee

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
)

// ValidateWebhook ensures that the provided request conforms to the
// format of a Gitee webhook and the payload can be validated with
// the provided hmac secret. It returns the event type, the event guid,
// the payload of the request, whether the webhook is valid or not,
// and finally the resultant HTTP status code
func ValidateWebhook(w http.ResponseWriter, r *http.Request, tokenGenerator func() []byte) (string, string, []byte, bool, int) {
	defer r.Body.Close()

	// Header checks: It must be a POST with an event type and a signature.
	if r.Method != http.MethodPost {
		responseHTTPError(w, http.StatusMethodNotAllowed, "405 Method not allowed")
		return "", "", nil, false, http.StatusMethodNotAllowed
	}
	eventType := r.Header.Get("X-Gitee-Event")
	if eventType == "" {
		responseHTTPError(w, http.StatusBadRequest, "400 Bad Request: Missing X-Gitee-Event Header")
		return "", "", nil, false, http.StatusBadRequest
	}
	eventGUID := r.Header.Get("X-Gitee-Timestamp")
	if eventGUID == "" {
		responseHTTPError(w, http.StatusBadRequest, "400 Bad Request: Missing X-Gitee-Timestamp Header")
		return "", "", nil, false, http.StatusBadRequest
	}
	sig := r.Header.Get("X-Gitee-Token")
	if sig == "" {
		responseHTTPError(w, http.StatusForbidden, "403 Forbidden: Missing X-Gitee-Token")
		return "", "", nil, false, http.StatusForbidden
	}
	contentType := r.Header.Get("content-type")
	if contentType != "application/json" {
		responseHTTPError(w, http.StatusBadRequest, "400 Bad Request: Hook only accepts content-type: application/json")
		return "", "", nil, false, http.StatusBadRequest
	}
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseHTTPError(w, http.StatusInternalServerError, "500 Internal Server Error: Failed to read request body")
		return "", "", nil, false, http.StatusInternalServerError
	}
	// Validate the payload with our HMAC secret.
	f := func(key string) string { return payloadSignature(eventGUID, key) }
	if !validatePayload(sig, tokenGenerator, f) {
		responseHTTPError(w, http.StatusForbidden, "403 Forbidden: Invalid X-Gitee-Token")
		return "", "", nil, false, http.StatusForbidden
	}

	return eventType, eventGUID, payload, true, http.StatusOK
}

func payloadSignature(timestamp, key string) string {
	mac := hmac.New(sha256.New, []byte(key))

	c := fmt.Sprintf("%s\n%s", timestamp, string(key))
	mac.Write([]byte(c))

	h := mac.Sum(nil)

	return base64.StdEncoding.EncodeToString(h)
}

func responseHTTPError(w http.ResponseWriter, statusCode int, response string) {
	logrus.WithFields(logrus.Fields{
		"response":    response,
		"status-code": statusCode,
	}).Debug(response)
	http.Error(w, response, statusCode)
}

func extractHmacs(tokenGenerator func() []byte) ([][]byte, error) {
	t := tokenGenerator()
	return [][]byte{t}, nil
}

func validatePayload(sig string, tokenGenerator func() []byte, ps func(string) string) bool {
	hmacs, err := extractHmacs(tokenGenerator)
	if err != nil {
		logrus.WithError(err).Error("couldn't unmarshal the hmac secret")
		return false
	}

	// If we have a match with any valid hmac, we can validate successfully.
	for _, key := range hmacs {
		if sig == ps(string(key)) {
			return true
		}
	}
	return false
}
