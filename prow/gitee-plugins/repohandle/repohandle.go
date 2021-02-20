package repohandle

import (
	"encoding/base64"
	"fmt"
	"k8s.io/test-infra/prow/interrupts"
	"strings"
	"time"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	prowConfig "k8s.io/test-infra/prow/config"
	plugins "k8s.io/test-infra/prow/gitee-plugins"
	"k8s.io/test-infra/prow/pluginhelp"
)

var cacheFilePath = "repo_handle_cache_config.json"

type getAllConf func() *plugins.Configurations

type repoHandle struct {
	rhc          *rhClient
	repoCfgCache *cacheProcessedFile
	sigCache     *sigCache
	getPluginCfg plugins.GetPluginConfig
	getAllConf   getAllConf
}

func NewRepoHandle(f plugins.GetPluginConfig, f1 getAllConf, gec giteeClient) plugins.Plugin {
	return &repoHandle{
		getPluginCfg: f,
		getAllConf:   f1,
		rhc:          &rhClient{giteeClient: gec},
		repoCfgCache: newCache(cacheFilePath),
		sigCache:     getSigInstance(),
	}
}

func (rh *repoHandle) PluginName() string {
	return "repoHandle"
}

func (rh *repoHandle) HelpProvider(_ []prowConfig.OrgRepo) (*pluginhelp.PluginHelp, error) {
	return nil, nil
}

func (rh *repoHandle) RegisterEventHandler(p plugins.Plugins) {
	pn := rh.PluginName()
	p.RegisterPushEventHandler(pn, rh.handlePushEvent)
}

func (rh repoHandle) NewPluginConfig() plugins.PluginConfig {
	return &configuration{}
}

//HandleDefaultTask handle this plugin default task, tasks not triggered by webhook
func (rh *repoHandle) HandleDefaultTask() {
	log := logrus.WithFields(
		logrus.Fields{
			"component":  rh.PluginName(),
			"event-type": "defaultTask",
		},
	)
	err := rh.repoCfgCache.cacheInit()
	if err != nil {
		log.Error(err)
	}
	interrupts.Tick(func() {
		rh.handleRepoDT(true, log)
	}, func() time.Duration {
		return time.Hour * 12
	})
	//go rh.handleRepoDT(true, log)
}

func (rh *repoHandle) handlePushEvent(e *sdk.PushEvent, log *logrus.Entry) error {
	if e == nil {
		return fmt.Errorf("hook event payload can not be nil")
	}
	c, err := rh.getPluginConfig()
	if err != nil {
		return err
	}
	//Check whether the sig configuration has changed
	if len(c.RepoHandler.SigFiles) == 0 {
		log.Debug("not find sig configuration ")
	} else {
		sigs, find := getSigCfgChangeFile(e, c.RepoHandler.SigFiles)
		if find {
			rh.triggerSigTask(sigs, log)
		}
	}

	//check whether the repository configuration has changed
	files, err := rh.getNeedHandleFiles()
	if err != nil {
		log.Error(err)
	} else if len(files) > 0 {
		idx, find := getRepoCfgChangeFile(e, files)
		if find {
			err = rh.triggerRepoTask(idx, files, log)
			if err != nil {
				log.Error(err)
			}
		}
	}

	//check  whether the owner configuration has changed
	if len(c.RepoHandler.OwnerFiles) == 0 {
		log.Debug("not find owner configuration")
	} else {
		oc, find := getOwnersChangeFile(e, c.RepoHandler.OwnerFiles)
		if find {
			rh.triggerOwnerTask(oc, log)
		}
	}

	return nil
}

func (rh *repoHandle) triggerSigTask(files []cfgFilePath, log *logrus.Entry) {
	var sigList []sig
	for _, v := range files {
		sigs, e := rh.handleSigConfig(v)
		if e != nil {
			log.Error(e)
		}
		sigList = append(sigList, sigs...)
	}
	if len(sigList) == 0 {
		return
	}
	mReps := rh.sigCache.modify(sigList)
	if len(mReps) == 0 {
		return
	}
	var err error
	for _, v := range mReps {
		err = rh.handleTriggerChangeRepoOwner(v, log)
		if err != nil {
			log.Error(err)
		}
	}
}

func (rh *repoHandle) handleTriggerChangeRepoOwner(fn string, log *logrus.Entry) error {
	if fn == "" || !strings.Contains(fn, "/") {
		return fmt.Errorf("illegal repository full name: %s", fn)
	}
	sp := strings.Split(fn, "/")
	if len(sp) != 2 {
		return fmt.Errorf("illegal repository full name: %s", fn)
	}
	repo, ex, err := rh.rhc.existRepo(sp[0], sp[1])
	if err != nil {
		return err
	}
	if !ex {
		return fmt.Errorf("%s does not exist ", fn)
	}
	err = rh.handleAddOwnerToRepo(&repo, log)
	return err
}

func (rh *repoHandle) triggerOwnerTask(sn []string, log *logrus.Entry) {
	if len(sn) == 0 {
		return
	}
	var mReps []string
	for _, v := range sn {
		sp := strings.Split(v, "/")
		for _, s := range sp {
			cReps := rh.sigCache.ownerChange(s)
			if len(cReps) > 0 {
				mReps = append(mReps, cReps...)
			}
		}
	}
	if len(mReps) == 0 {
		return
	}
	var err error
	for _, v := range mReps {
		err = rh.handleTriggerChangeRepoOwner(v, log)
		if err != nil {
			log.Error(err)
		}
	}
}

func (rh *repoHandle) triggerRepoTask(idx []int, files []cfgFilePath, log *logrus.Entry) error {
	if len(files) == 0 || len(idx) == 0 {
		return nil
	}
	var err error
	for k := range idx {
		err = rh.handleRepoConfigFile(&files[k], log)
		if err != nil {
			log.Error(err)
		}
	}
	return rh.repoCfgCache.saveCache(files)
}

func (rh *repoHandle) getPluginConfig() (*configuration, error) {
	cfg := rh.getPluginCfg(rh.PluginName())
	if cfg == nil {
		return nil, fmt.Errorf("can't find the configuration")
	}

	c, ok := cfg.(*configuration)
	if !ok {
		return nil, fmt.Errorf("can't convert to configuration")
	}
	return c, nil
}

func (rh *repoHandle) handleRepoDT(loadSig bool, log *logrus.Entry) {
	if loadSig {
		rh.loadSigConfig(log)
	}
	files, err := rh.getNeedHandleFiles()
	if err != nil {
		log.Error(err)
		return
	}
	if len(files) == 0 {
		return
	}
	for k := range files {
		err = rh.handleRepoConfigFile(&files[k], log)
		if err != nil {
			log.Error(err)
		}
	}
	err = rh.repoCfgCache.saveCache(files)
	if err != nil {
		log.Error(err)
	}
}

func (rh *repoHandle) handleRepoConfigFile(file *cfgFilePath, log *logrus.Entry) error {
	content, err := rh.rhc.getRealPathContent(file.Owner, file.Repo, file.Path, file.Ref)
	if err != nil {
		return err
	}
	if content.Sha == "" || content.Content == "" || content.Sha == file.Hash {
		log.Info(fmt.Sprintf("%s/%s/%s configuration does not need to be processed", file.Owner, file.Repo, file.Path))
		return nil
	}
	decodeBytes, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return err
	}
	rc := Repos{}
	err = yaml.UnmarshalStrict(decodeBytes, &rc)
	if err != nil {
		return err
	}
	if rc.Community == "" || len(rc.Repositories) == 0 {
		return fmt.Errorf("repos configuration error")
	}
	canCache := true
	for _, v := range rc.Repositories {
		status, err := rh.handleAddRepository(rc.Community, v, log)
		if !status {
			canCache = status
		}
		if err != nil {
			log.Error(err)
		}
	}
	if canCache {
		file.Hash = content.Sha
	}
	return nil
}

func (rh *repoHandle) handleAddRepository(community string, repository Repository, log *logrus.Entry) (bool, error) {
	dc := true
	repo, ex, err := rh.rhc.existRepo(community, *repository.Name)
	if err != nil {
		return false, err
	}
	//handle rename repo first
	if !ex && repository.RenameFrom != nil && (*repository.RenameFrom) != "" {
		rnRepo, exist, err := rh.rhc.existRepo(community, *repository.RenameFrom)
		if err != nil {
			return false, err
		}
		if !exist {
			return false, fmt.Errorf("repository defined by rename_from does not exist: %s ", *repository.RenameFrom)
		}
		err = rh.rhc.updateRepoName(community, rnRepo.Name, *repository.Name)
		if err != nil {
			return false, err
		}
	}
	if !ex {
		//add repo on gitee
		repo, err = rh.rhc.createRepo(community, *repository.Name, *repository.Description, *repository.Type, repository.AutoInit)
		if err != nil {
			return false, err
		}
		// add branch on repo
		if len(repository.ProtectedBranches) > 0 {
			for _, v := range repository.ProtectedBranches {
				if v != "" && v != repo.DefaultBranch {
					_, err = rh.rhc.giteeClient.CreateBranch(community, repo.Name, repo.DefaultBranch, v)
					if err != nil {
						dc = false
						log.Error(err)
					}
				}
			}
		}

	}
	//setting branch
	err = rh.handleRepoBranchProtected(community, repository)
	if err != nil {
		dc = false
		log.Error(err)
	}
	//repo setting
	err = rh.handleRepositorySetting(&repo, repository)
	if err != nil {
		dc = false
		log.Error(err)
	}
	// add owner
	err = rh.handleAddOwnerToRepo(&repo, log)
	if err != nil {
		dc = false
	}
	return dc, err
}

func (rh *repoHandle) handleRepositorySetting(repo *sdk.Project, repository Repository) error {
	if repo == nil {
		return fmt.Errorf("gitee %s repository is nil", *repository.Name)
	}
	owner := repo.Namespace.Path
	if owner == "" {
		return fmt.Errorf("repository %s information not obtained", *repository.Name)
	}
	exceptType := "private" == *repository.Type
	exceptCommentable := repository.IsCommentable()
	typeChange := repo.Private != exceptType
	commentChange := repo.CanComment != exceptCommentable
	if typeChange || commentChange {
		pt := repo.Private
		pc := repo.CanComment
		if typeChange {
			pt = exceptType
		}
		if commentChange {
			pc = exceptCommentable
		}
		err := rh.rhc.updateRepoCommentOrType(owner, *repository.Name, pc, pt)
		return err
	}
	return nil
}

func (rh *repoHandle) handleRepoBranchProtected(community string, repository Repository) error {
	// if the branches are defined in the repositories, it means that
	// all the branches defined in the community will not inherited by repositories
	branch, err := rh.rhc.GetRepoAllBranch(community, *repository.Name)
	if err != nil {
		return err
	}
	cbm := make(map[string]int, len(branch))
	for k, v := range branch {
		cbm[v.Name] = k
	}
	nbm := make(map[string]string, len(repository.ProtectedBranches))
	for _, v := range repository.ProtectedBranches {
		nbm[v] = v
	}
	//remove protected config dose not exist in current branches when branch is protected
	for k, v := range cbm {
		if branch[v].Protected {
			if _, exist := nbm[k]; !exist {
				err = rh.rhc.CancelBranchProtected(community, *repository.Name, k)
				if err == nil {
					branch[v].Protected = false
				}
			}
		}
	}
	//add protected current config branch on repository
	for k := range nbm {
		if v, exist := cbm[k]; exist && branch[v].Protected == false {
			_, err = rh.rhc.SetBranchProtected(community, *repository.Name, k)
			if err != nil {
				logrus.Println(err)
			}
		}
	}
	return nil
}

//getOwnerPluginOrg Organization that owns this plugin
func (rh *repoHandle) getOwnerPluginOrg() map[string]struct{} {
	conf := rh.getAllConf()
	empty := struct{}{}
	orgs := make(map[string]struct{}, 0)
	if conf == nil {
		return orgs
	}
	for k := range conf.Plugins {
		contain := false
		if len(conf.Plugins[k]) == 0 {
			continue
		}
		for _, p := range conf.Plugins[k] {
			if p == rh.PluginName() {
				contain = true
				break
			}
		}
		if !contain {
			continue
		}
		orgs[k] = empty
	}
	return orgs
}

func (rh *repoHandle) getNeedHandleFiles() ([]cfgFilePath, error) {
	var repoFiles []cfgFilePath
	c, err := rh.getPluginConfig()
	if err != nil {
		return repoFiles, err
	}
	cfs := getLegalCfgFile(rh.getOwnerPluginOrg(), c.RepoHandler.RepoFiles)
	for _, f := range cfs {
		if f.Owner != "" && f.Repo != "" && f.Path != "" {
			repoFiles = append(repoFiles, f)
		}
	}
	cacheConfig, err := rh.repoCfgCache.loadCache()
	if err != nil {
		return repoFiles, nil
	}
	if len(cacheConfig) > 0 {
		for k := range repoFiles {
			for _, v := range cacheConfig {
				if repoFiles[k].equal(v) {
					repoFiles[k].Hash = v.Hash
				}
			}
		}
	}
	return repoFiles, nil
}

func (rh *repoHandle) loadSigConfig(log *logrus.Entry) {
	c, err := rh.getPluginConfig()
	if err != nil {
		log.Error(err)
		return
	}
	if len(c.RepoHandler.SigFiles) == 0 {
		log.Debug("no sig config")
		return
	}
	var sl []sig
	for _, v := range c.RepoHandler.SigFiles {
		s, err := rh.handleSigConfig(v)
		if err != nil {
			log.Error(err)
			continue
		}
		sl = append(sl, s...)
	}
	rh.sigCache.init(sl)
}

func (rh *repoHandle) handleSigConfig(file cfgFilePath) ([]sig, error) {
	var sl []sig
	content, err := rh.rhc.getRealPathContent(file.Owner, file.Repo, file.Path, file.Ref)
	if err != nil {
		return sl, err
	}
	decodeBytes, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return sl, err
	}
	sc := sigCfg{}
	err = yaml.UnmarshalStrict(decodeBytes, &sc)
	if err != nil {
		return sl, err
	}
	sl = sc.Sigs
	return sl, nil
}

func (rh *repoHandle) loadOwners(sn []string, log *logrus.Entry) map[string]struct{} {
	rto := make(map[string]struct{})
	rtoV := struct{}{}
	c, err := rh.getPluginConfig()
	if err != nil {
		log.Error(err)
		return rto
	}
	for _, v := range sn {
		cs := getOwnerFileCfgBySigName(c.RepoHandler.OwnerFiles, v)
		for _, c := range cs {
			owners, err := rh.handleOwnerConfig(c)
			if err != nil {
				log.Error(err)
				continue
			}
			for _, o := range owners {
				rto[o] = rtoV
			}
		}
	}
	return rto
}

func (rh *repoHandle) handleOwnerConfig(file cfgFilePath) ([]string, error) {
	var sl []string
	content, err := rh.rhc.getRealPathContent(file.Owner, file.Repo, file.Path, file.Ref)
	if err != nil {
		return sl, err
	}
	decodeBytes, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return sl, err
	}
	sc := owner{}
	err = yaml.UnmarshalStrict(decodeBytes, &sc)
	return sc.Maintainers, err
}

func (rh *repoHandle) handleAddOwnerToRepo(repo *sdk.Project, log *logrus.Entry) error {
	if repo == nil {
		return fmt.Errorf("handleAddOwnerToRepo: the repo is nil")
	}
	sigList := rh.sigCache.loadSigName(repo.FullName)
	omap := rh.loadOwners(sigList, log)
	if len(omap) == 0 {
		return nil
	}
	for _, v := range repo.Members {
		if _, ok := omap[v]; ok {
			delete(omap, v)
		}
	}
	var err error
	for k := range omap {
		err = rh.rhc.AddRepositoryMember(repo.Namespace.Path, repo.Name, k, "pull")
		if err != nil {
			log.WithError(err).Error(fmt.Errorf("add member %s fail", k))
		}
	}
	return nil
}
