package checkpr

import (
	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"

	prowConfig "k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pluginhelp"
)

type cpClient interface {
	UpdatePullRequest(org, repo string, number int32, param sdk.PullRequestUpdateParam) (sdk.PullRequest, error)
	GetGiteePullRequest(org, repo string, number int) (sdk.PullRequest, error)
}

type checkPr struct {
	ghc             cpClient
	getPluginConfig plugins.GetPluginConfig
}

func NewCheckPr(f plugins.GetPluginConfig, gec gitee.Client) plugins.Plugin {
	return &checkPr{gec, f}
}

func (cp *checkPr) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: `The checkpr plugin will remove the min num of reviewers and testers designated by the PR author.`,
	}
	return pluginHelp, nil
}

func (cp *checkPr) PluginName() string {
	return "checkpr"
}

func (cp *checkPr) NewPluginConfig() plugins.PluginConfig {
	return nil
}

func (cp *checkPr) RegisterEventHandler(p plugins.Plugins) {
	p.RegisterPullRequestHandler(cp.PluginName(), cp.removeMinNumReviewerAndTester)
}

func (cp *checkPr) removeMinNumReviewerAndTester(e *sdk.PullRequestEvent, log *logrus.Entry) error {
	action := plugins.ConvertPullRequestAction(e)
	if action == github.PullRequestActionClosed {
		return nil
	}

	org := e.Repository.Namespace
	repo := e.Repository.Path
	number := e.PullRequest.Number

	pr, err := cp.ghc.GetGiteePullRequest(org, repo, int(number))
	if err != nil {
		return err
	}
	if pr.AssigneesNumber == 0 && pr.TestersNumber == 0 {
		return nil
	}

	changeNum := int32(0)
	param := sdk.PullRequestUpdateParam{AssigneesNumber: &changeNum, TestersNumber: &changeNum}
	_, err = cp.ghc.UpdatePullRequest(org, repo, number, param)
	return err
}
