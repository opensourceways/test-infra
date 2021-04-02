package gitee

import (
	"k8s.io/test-infra/prow/apis/prowjobs/v1"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/crier/reporters/github"
	jobreporter "k8s.io/test-infra/prow/job-reporter"
)

type reporter struct {
	c       *github.Client
	gec     giteeClient
	botname string
}

func NewReporter(gec giteeClient, cfg config.Getter, reportAgent v1.ProwJobAgent) (*reporter, error) {
	botname, err := gec.BotName()
	if err != nil {
		return nil, err
	}

	ghc := &ghclient{giteeClient: gec}
	c := github.NewReporter(ghc, cfg, reportAgent)
	return &reporter{c: c, gec: gec, botname: botname}, nil
}

// GetName returns the name of the reporter
func (r *reporter) GetName() string {
	return "gitee-reporter"
}

func (r *reporter) ShouldReport(pj *v1.ProwJob) bool {
	if IsGiteeJob(pj) {
		return r.c.ShouldReport(pj)
	}
	return false
}

func (r *reporter) Report(pj *v1.ProwJob) ([]*v1.ProwJob, error) {
	if pj.Spec.Refs.Pulls != nil && len(pj.Spec.Refs.Pulls) == 1 {
		return r.c.Report1(pj, &ghclient{
			giteeClient: r.gec,
			botname:     r.botname,
			baseSHA:     pj.Spec.Refs.BaseSHA,
			prNumber:    pj.Spec.Refs.Pulls[0].Number,
		})
	}

	return r.c.Report(pj)
}

func IsGiteeJob(pj *v1.ProwJob) bool {
	if pj.Annotations != nil {
		if v, ok := pj.Annotations[jobreporter.JobPlatformAnnotation]; ok {
			return (v == "gitee")
		}
	}
	return false
}
