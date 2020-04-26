package assign

import (
	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/prow/github"
)

func HandleAssign(e github.GenericCommentEvent, gc githubClient, onAddFailure func(mu github.MissingUsers) string, log *logrus.Entry) error {
	h := newAssignHandler(e, gc, log)
	h.addFailureResponse = onAddFailure
	return handle(h)
}

var HelpProvider = helpProvider
