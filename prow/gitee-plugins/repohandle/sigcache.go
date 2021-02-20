package repohandle

import "sync"

var (
	sigInstance *sigCache
	sigOnce     sync.Once
)

type sigCache struct {
	sigMap map[string][]string
	sync.RWMutex
}

func getSigInstance() *sigCache {
	sigOnce.Do(func() {
		sigInstance = &sigCache{}
		sigInstance.sigMap = make(map[string][]string, 0)
	})
	return sigInstance
}

func (sc *sigCache) init(sigList []sig) {
	sc.Lock()
	sc.sigMap = make(map[string][]string, 0)
	for _, v := range sigList {
		sc.sigMap[v.Name] = v.Repositories
	}
	sc.Unlock()
}

func (sc *sigCache) modify(sigList []sig) []string {
	sc.Lock()
	defer sc.Unlock()
	if sc.sigMap == nil {
		sc.sigMap = make(map[string][]string, 0)
	}
	repos := make([]string, 0)
	for _, v := range sigList {
		if rs, ok := sc.sigMap[v.Name]; !ok {
			sc.sigMap[v.Name] = v.Repositories
			repos = append(repos, v.Repositories...)
		} else {
			nc := false
			if len(v.Repositories) != len(rs) {
				nc = true
			}

			for _, rep := range v.Repositories {
				ex := false
				for _, re := range rs {
					if re == rep {
						ex = true
						break
					}
				}
				if !ex {
					repos = append(repos, rep)
					nc = true
				}
			}
			if nc {
				sc.sigMap[v.Name] = v.Repositories
			}
		}
	}
	return repos
}

func (sc *sigCache) loadSigName(repo string) []string {
	sc.RLock()
	defer sc.RUnlock()
	var sigNames []string
	if repo == "" || len(sc.sigMap) == 0 {
		return sigNames
	}
	for k, v := range sc.sigMap {
		for _, r := range v {
			if r == repo {
				sigNames = append(sigNames, k)
			}
		}
	}
	return sigNames
}

//ownerChange When the owner configuration file changes,
// call this method to get the repository that needs to be processed
func (sc *sigCache) ownerChange(sn string) []string {
	sc.RLock()
	defer sc.RUnlock()
	var cRepo []string
	if sn == "" || len(sc.sigMap) == 0 {
		return cRepo
	}
	if v, ok := sc.sigMap[sn]; ok {
		cRepo = v
	}
	return cRepo
}
