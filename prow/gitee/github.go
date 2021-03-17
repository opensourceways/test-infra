package gitee

import (
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"k8s.io/test-infra/prow/github"
)

func ConvertGiteePRComment(i sdk.PullRequestComments) github.IssueComment {
	ct, _ := time.Parse(time.RFC3339, i.CreatedAt)
	ut, _ := time.Parse(time.RFC3339, i.UpdatedAt)

	return github.IssueComment{
		ID:        int(i.Id),
		Body:      i.Body,
		User:      github.User{Login: i.User.Login},
		HTMLURL:   i.HtmlUrl,
		CreatedAt: ct,
		UpdatedAt: ut,
	}
}

func ConvertGiteePR(v *sdk.PullRequest) *github.PullRequest {
	r := github.PullRequest{
		Head: github.PullRequestBranch{
			SHA: v.Head.Sha,
			Ref: v.Head.Ref,
		},
		Base: github.PullRequestBranch{
			Ref: v.Base.Ref,
			SHA: v.Base.Sha,
			Repo: github.Repo{
				Name: v.Base.Repo.Path,
				Owner: github.User{
					Login: v.Base.Repo.Namespace.Path,
				},
				HTMLURL:  v.Base.Repo.HtmlUrl,
				FullName: v.Base.Repo.FullName,
			},
		},
		User: github.User{
			Login:   v.User.Login,
			HTMLURL: v.User.HtmlUrl,
		},

		Number:  int(v.Number),
		HTMLURL: v.HtmlUrl,
		State:   v.State,
		Body:    v.Body,
		Title:   v.Title,
		ID:      int(v.Id),
	}
	return &r
}
