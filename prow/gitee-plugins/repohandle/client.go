package repohandle

import (
	sdk "gitee.com/openeuler/go-gitee/gitee"
	"strconv"
	"strings"
)

type giteeClient interface {
	GetPathContent(owner, repo, path, ref string) (sdk.Content, error)
	GetGiteeRepo(org, repo string) (sdk.Project, error)
	CreateRepo(owner string, param sdk.RepositoryPostParam) (sdk.Project, error)
	UpdateRepo(owner, repo string, bp sdk.RepoPatchParam) error
	GetRepoAllBranch(owner, repo string) ([]sdk.Branch, error)
	CreateBranch(owner, repo, ref, bName string) (sdk.CompleteBranch, error)
	CancelBranchProtected(owner, repo, bName string) error
	SetBranchProtected(owner, repo, bName string) (sdk.CompleteBranch, error)
	AddRepositoryMember(owner, repo, username, permission string) error
}

type rhClient struct {
	giteeClient
}

func (rh *rhClient) getRealPathContent(owner, repo, path, ref string) (sdk.Content, error) {
	return rh.GetPathContent(owner, repo, path, ref)
}

func (rh *rhClient) existRepo(org, repo string) (sdk.Project, bool, error) {
	giteRepo, err := rh.GetGiteeRepo(org, repo)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return giteRepo, false, nil
		}
		return giteRepo, false, err
	}
	return giteRepo, true, err
}

func (rh *rhClient) updateRepoName(org, repo, reName string) error {
	param := sdk.RepoPatchParam{Name: reName, Path: reName}
	return rh.UpdateRepo(org, repo, param)
}

func (rh *rhClient) updateRepoCommentOrType(org, repo string, comment, private bool) error {
	param := sdk.RepoPatchParam{CanComment: strconv.FormatBool(comment), Private: strconv.FormatBool(private)}
	return rh.UpdateRepo(org, repo, param)
}

func (rh *rhClient) createRepo(org, repo, description, t string, autoInit bool) (sdk.Project, error) {
	param := sdk.RepositoryPostParam{Name: repo, Description: description, HasIssues: true,
		CanComment: true, HasWiki: true, AutoInit: autoInit}
	param.Private = t == "private"
	return rh.CreateRepo(org, param)
}
