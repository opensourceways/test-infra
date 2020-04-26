package plugins

import (
	originp "k8s.io/test-infra/prow/plugins"
)

func init() {
	// TODO: add the url of all commands
	originp.ExAboutThisBotWithoutCommands = "Instructions for interacting with me using PR comments are available here. If you have questions or suggestions related to my behavior, please file an issue against the [opensourceways/test-infra](https://github.com/opensourceways/test-infra/issues/new?title=Prow%20issue:) repository."
}
