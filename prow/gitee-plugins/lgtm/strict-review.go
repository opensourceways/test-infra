package lgtm

import (
	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/plugins"
	originl "k8s.io/test-infra/prow/plugins/lgtm"
	"k8s.io/test-infra/prow/repoowners"
)

func HandleStrictLGTMPREvent(gc *ghclient, e *github.PullRequestEvent, minReview int) error {
	pr := e.PullRequest
	org := pr.Base.Repo.Owner.Login
	repo := pr.Base.Repo.Name
	prNumber := pr.Number
	sha, err := getHashTree(gc, org, repo, pr.Head.SHA)
	if err != nil {
		return err
	}

	var n *notification
	needRemoveLabel := false

	switch e.Action {
	case github.PullRequestActionOpened:
		n = &notification{
			treeHash:  sha,
			minReview: minReview,
		}
		filenames, err := originl.GetChangedFiles(gc, org, repo, prNumber)
		if err != nil {
			return err
		}

		n.ResetDirs(genDirs(filenames))

	case github.PullRequestActionSynchronize:
		v, prChanged, err := LoadLGTMnotification(gc, org, repo, prNumber, sha)
		if err != nil {
			return err
		}

		if !prChanged {
			return nil
		}

		n = v
		n.minReview = minReview
		needRemoveLabel = true

	default:
		return nil
	}

	if err := n.WriteComment(gc, org, repo, prNumber, false); err != nil {
		return err
	}

	if needRemoveLabel {
		return gc.RemoveLabel(org, repo, prNumber, originl.LGTMLabel)
	}
	return nil
}

// skipCollaborators && strictReviewer
func HandleStrictLGTMComment(gc *ghclient, oc repoowners.Interface, log *logrus.Entry, wantLGTM bool, e *sdk.NoteEvent, minReview int) error {
	pr := e.PullRequest
	s := &strictReview{
		gc:  gc,
		oc:  oc,
		log: log,

		org:      e.Repository.Namespace,
		repo:     e.Repository.Path,
		prAuthor: pr.User.Login,
		prNumber: int(pr.Number),
	}

	sha, err := getHashTree(gc, s.org, s.repo, pr.Head.Sha)
	if err != nil {
		return err
	}
	s.treeHash = sha

	noti, _, err := LoadLGTMnotification(gc, s.org, s.repo, s.prNumber, s.treeHash)
	if err != nil {
		return err
	}
	noti.minReview = minReview

	validReviewers, err := s.fileReviewers()
	if err != nil {
		return err
	}

	hasLGTM, err := s.hasLGTMLabel()
	if err != nil {
		return err
	}

	if !wantLGTM {
		return s.handleLGTMCancel(noti, validReviewers, e, hasLGTM)
	}

	return s.handleLGTM(noti, validReviewers, e, hasLGTM)
}

type strictReview struct {
	log *logrus.Entry
	gc  *ghclient
	oc  repoowners.Interface

	org      string
	repo     string
	treeHash string
	prAuthor string
	prNumber int
}

func (sr *strictReview) handleLGTMCancel(noti *notification, validReviewers map[string]sets.String, e *sdk.NoteEvent, hasLabel bool) error {
	commenter := e.Comment.User.Login

	if commenter != sr.prAuthor && !isReviewer(validReviewers, commenter) {
		noti.AddOpponent(commenter, false)

		return sr.writeComment(noti, hasLabel)
	}

	if commenter == sr.prAuthor {
		noti.ResetConsentor()
		noti.ResetOpponents()
	} else {
		// commenter is not pr author, but is reviewr
		// I don't know which part of code commenter thought it is not good
		// Maybe it is directory of which he is reviewer, maybe other parts.
		// So, it simply sets all the codes need review again. Because the
		// lgtm label needs no reviewer say `/lgtm cancel`
		noti.AddOpponent(commenter, true)
	}

	filenames := make([]string, 0, len(validReviewers))
	for k := range validReviewers {
		filenames = append(filenames, k)
	}
	noti.ResetDirs(genDirs(filenames))

	err := sr.writeComment(noti, false)
	if err != nil {
		return err
	}

	if hasLabel {
		return sr.removeLabel()
	}
	return nil
}

func (sr *strictReview) handleLGTM(noti *notification, validReviewers map[string]sets.String, e *sdk.NoteEvent, hasLabel bool) error {
	comment := e.Comment
	commenter := comment.User.Login

	if commenter == sr.prAuthor {
		resp := "you cannot LGTM your own PR."
		return sr.gc.CreateComment(
			sr.org, sr.repo, sr.prNumber,
			plugins.FormatResponseRaw(comment.Body, comment.HtmlUrl, commenter, resp))
	}

	consentors := noti.GetConsentors()
	if _, ok := consentors[commenter]; ok {
		// add /lgtm repeatedly
		return nil
	}

	ok := isReviewer(validReviewers, commenter)
	noti.AddConsentor(commenter, ok)

	if !ok {
		return sr.writeComment(noti, hasLabel)
	}

	resetReviewDir(validReviewers, noti)

	ok = canAddLgtmLabel(noti)
	if err := sr.writeComment(noti, ok); err != nil {
		return err
	}

	if ok && !hasLabel {
		return sr.addLabel()
	}

	if !ok && hasLabel {
		return sr.removeLabel()
	}

	return nil
}

func (sr *strictReview) fileReviewers() (map[string]sets.String, error) {
	ro, err := originl.LoadRepoOwners(sr.gc, sr.oc, sr.org, sr.repo, sr.prNumber)
	if err != nil {
		return nil, err
	}

	filenames, err := originl.GetChangedFiles(sr.gc, sr.org, sr.repo, sr.prNumber)
	if err != nil {
		return nil, err
	}

	m := map[string]sets.String{}
	for _, filename := range filenames {
		m[filename] = ro.Approvers(filename).Union(ro.Reviewers(filename))
	}

	return m, nil
}

func (sr *strictReview) writeComment(noti *notification, ok bool) error {
	return noti.WriteComment(sr.gc, sr.org, sr.repo, sr.prNumber, ok)
}

func (sr *strictReview) hasLGTMLabel() (bool, error) {
	labels, err := sr.gc.GetIssueLabels(sr.org, sr.repo, sr.prNumber)
	if err != nil {
		return false, err
	}
	return github.HasLabel(originl.LGTMLabel, labels), nil
}

func (sr *strictReview) removeLabel() error {
	return sr.gc.RemoveLabel(sr.org, sr.repo, sr.prNumber, originl.LGTMLabel)
}

func (sr *strictReview) addLabel() error {
	return sr.gc.AddLabel(sr.org, sr.repo, sr.prNumber, originl.LGTMLabel)
}

func canAddLgtmLabel(noti *notification) bool {
	for _, v := range noti.GetOpponents() {
		if v {
			// there are reviewers said `/lgtm cancel`
			return false
		}
	}
	nrOk := noti.getValidReviewCount() >= noti.minReview
	d := noti.GetDirs()
	return len(d) == 0 && nrOk
}

func isReviewer(validReviewers map[string]sets.String, commenter string) bool {
	commenter = github.NormLogin(commenter)

	for _, rs := range validReviewers {
		if rs.Has(commenter) {
			return true
		}
	}

	return false
}

func resetReviewDir(validReviewers map[string]sets.String, noti *notification) {
	consentors := noti.GetConsentors()
	reviewers := make([]string, 0, len(consentors))
	for k, v := range consentors {
		if v {
			reviewers = append(reviewers, github.NormLogin(k))
		}
	}

	needReview := map[string]bool{}
	for filename, rs := range validReviewers {
		if !rs.HasAny(reviewers...) {
			needReview[filename] = true
		}
	}

	if len(needReview) != 0 {
		noti.ResetDirs(genDirs(mapKeys(needReview)))
	} else {
		noti.ResetDirs(nil)
	}
}

func getHashTree(gc *ghclient, org, repo, headSHA string) (string, error) {
	commit, err := gc.GetSingleCommit(org, repo, headSHA)
	if err != nil {
		return "", err
	}

	return commit.Commit.Tree.SHA, nil
}
