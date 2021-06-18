package gitee

import (
	"strconv"

	sdk "gitee.com/openeuler/go-gitee/gitee"
)

//GetOwnerAndRepoByPREvent obtain the owner and repository name from the pullrequest's event
func GetOwnerAndRepoByPREvent(pre *sdk.PullRequestEvent) (string, string) {
	return pre.Repository.Namespace, pre.Repository.Path
}

//GetOwnerAndRepoByIssueEvent obtain the owner and repository name from the issue's event
func GetOwnerAndRepoByIssueEvent(issue *sdk.IssueEvent) (string, string) {
	return issue.Repository.Namespace, issue.Repository.Path
}

func GetLabelFromEvent(labels []sdk.LabelHook) map[string]bool {
	m := map[string]bool{}
	for i := range labels {
		m[labels[i].Name] = true
	}
	return m
}

func stringToInt(num string) int {
	rs, err := strconv.Atoi(num)
	if err != nil {
		return 0
	}
	return rs
}
