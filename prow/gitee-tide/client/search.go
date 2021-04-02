package client

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	githubql "github.com/shurcooL/githubv4"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/gitee"
	tide "k8s.io/test-infra/prow/gitee-tide"
)

func (c *ghclient) searchPR(q config.TideQuery, opt gitee.ListPullRequestOpt) ([]tide.PullRequest, error) {
	rprs := []tide.PullRequest{}
	rprm := map[string]bool{}

	// gitee api will return prs which have one of labels.
	opt.Labels = q.Labels
	r := c.getSearchRepos(q)
	for v := range r {
		org, repo := splitToOrgRepo(v)
		if org == "" {
			continue
		}

		prs, err := c.GetPullRequests(org, repo, opt)
		if err != nil {
			continue
		}

		for _, pr := range prs {
			k := prKey(pr)

			if filterPR(q, pr) || !pr.Mergeable || pr.AssigneesNumber != 0 || pr.TestersNumber != 0 || rprm[k] {
				continue
			}
			rprm[k] = true
			rprs = append(rprs, convertToPullRequest(pr))
		}
	}
	return rprs, nil
}

// TODO
func (c *ghclient) findReposOfOrg(org string) ([]string, error) {
	return []string{}, nil
}

func (c *ghclient) getSearchRepos(q config.TideQuery) sets.String {
	em := map[string]bool{}
	for _, k := range q.ExcludedRepos {
		em[k] = true
	}

	r := sets.NewString(q.Repos...)

	for _, org := range q.Orgs {
		repos, err := c.findReposOfOrg(org)
		if err != nil {
			continue
		}

		toAdd := make([]string, 0, len(repos))

		for _, repo := range repos {
			s := orgRepo(org, repo)

			if _, ok := em[s]; !ok {
				toAdd = append(toAdd, s)
			}
		}
		r.Insert(toAdd...)
	}
	return r
}

func filterPR(q config.TideQuery, pr sdk.PullRequest) bool {
	if len(q.Labels) > 0 {
		m := map[string]bool{}
		for i := range pr.Labels {
			m[pr.Labels[i].Name] = true
		}

		for _, l := range q.Labels {
			if !m[l] {
				return true
			}
		}
	}

	return false
}

func prKey(pr sdk.PullRequest) string {
	return fmt.Sprintf("%s/%v", pr.Base.Repo.FullName, pr.Number)
}

func orgRepo(org, repo string) string {
	return fmt.Sprintf("%s/%s", org, repo)
}

func splitToOrgRepo(s string) (string, string) {
	v := strings.Split(s, "/")
	if len(v) == 2 {
		return v[0], v[1]
	}
	return "", ""
}

func convertToPullRequest(pr sdk.PullRequest) tide.PullRequest {
	mergeable := githubql.MergeableStateMergeable
	if !pr.Mergeable {
		mergeable = githubql.MergeableStateConflicting
	}
	ut, _ := time.Parse(time.RFC3339, pr.UpdatedAt)

	r := tide.PullRequest{
		Number:      githubql.Int(pr.Number),
		HeadRefName: githubql.String(pr.Head.Ref),
		HeadRefOID:  githubql.String(pr.Head.Sha),
		Mergeable:   mergeable,
		Body:        githubql.String(pr.Body),
		Title:       githubql.String(pr.Title),
		UpdatedAt:   githubql.DateTime{Time: ut},
	}

	r.Author.Login = githubql.String(pr.User.Login)
	r.BaseRef.Name = githubql.String(pr.Base.Ref)
	r.Repository.Name = githubql.String(pr.Base.Repo.Path)
	r.Repository.NameWithOwner = githubql.String(pr.Base.Repo.FullName)
	r.Repository.Owner.Login = githubql.String(pr.Base.Repo.Namespace.Path)

	if pr.Milestone != nil {
		r.Milestone = &struct {
			Title githubql.String
		}{Title: githubql.String(pr.Milestone.Title)}
	}

	ls := pr.Labels
	if ls != nil && len(ls) > 0 {
		v := make([]struct{ Name githubql.String }, 0, len(ls))
		for _, i := range ls {
			v = append(v, struct{ Name githubql.String }{Name: githubql.String(i.Name)})
		}
		r.Labels.Nodes = v
	}
	return r
}

func parseQueryStr(q string) (bool, config.TideQuery, gitee.ListPullRequestOpt) {
	opt := gitee.ListPullRequestOpt{}
	qe := config.TideQuery{
		Orgs:             []string{},
		Repos:            []string{},
		ExcludedRepos:    []string{},
		ExcludedBranches: []string{},
		IncludedBranches: []string{},
		Labels:           []string{},
		MissingLabels:    []string{},
	}

	f := func(k, v string) {
		handlePRQuery(k, v, &qe, &opt)
	}
	isIssue := (strings.Index(q, "is:issue") != -1)
	if isIssue {
		f = func(k, v string) {}
	}

	re := regexp.MustCompile(fmt.Sprintf("%s:\"?%s\"?", "(.*)", "([^\"]*)"))
	a := strings.Split(q, " ")
	for _, item := range a {
		m := re.FindStringSubmatch(item)
		if m != nil {
			f(m[1], m[2])
		}
	}

	return !isIssue, qe, opt
}

func handlePRQuery(k, v string, q *config.TideQuery, opt *gitee.ListPullRequestOpt) {
	switch k {
	case "state":
		opt.State = v
	case "sort":
		item := strings.Split(v, "-")
		opt.Sort = item[0]
		opt.Direction = item[1]
	default:
		handleTideQuery(k, v, q)
	}
}

func handleTideQuery(k, v string, q *config.TideQuery) {
	switch k {
	case "org":
		q.Orgs = append(q.Orgs, v)
	case "repo":
		q.Repos = append(q.Repos, v)
	case "-repo":
		q.ExcludedRepos = append(q.ExcludedRepos, v)
	case "author":
		q.Author = v
	case "-base":
		q.ExcludedBranches = append(q.ExcludedBranches, v)
	case "base":
		q.IncludedBranches = append(q.IncludedBranches, v)
	case "label":
		q.Labels = append(q.Labels, v)
	case "-label":
		q.MissingLabels = append(q.MissingLabels, v)
	case "milestone":
		q.Milestone = v
	case "review":
		q.ReviewApprovedRequired = true
	}
}
