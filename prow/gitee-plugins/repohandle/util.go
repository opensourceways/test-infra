package repohandle

import (
	"strings"

	sdk "gitee.com/openeuler/go-gitee/gitee"
)

const ownerCfgPathSplit = "*"
const judgePathContain = ownerCfgPathSplit + "/"

func getRepoCfgChangeFile(e *sdk.PushEvent, files []cfgFilePath) ([]int, bool) {
	fns := getAllChangeFiles(e)
	var fIdx []int
	if len(fns) == 0 {
		return fIdx, false
	}
	find := false
	for k, v := range files {
		if e.Repository.Namespace != v.Owner || e.Repository.Name != v.Repo || e.Ref == nil {
			continue
		}
		ref := v.Ref
		if ref == "" {
			ref = "master"
		}
		if !strings.Contains(*e.Ref, ref) {
			continue
		}
		if _, ex := fns[v.Path]; ex {
			find = true
			fIdx = append(fIdx, k)
		}
	}
	return fIdx, find
}

func getSigCfgChangeFile(e *sdk.PushEvent, files []cfgFilePath) ([]cfgFilePath, bool) {
	fns := getAllChangeFiles(e)
	var cfs []cfgFilePath
	if len(fns) == 0 {
		return cfs, false
	}
	find := false
	for _, v := range files {
		if e.Repository.Namespace != v.Owner || e.Repository.Name != v.Repo || e.Ref == nil {
			continue
		}
		ref := v.Ref
		if ref == "" {
			ref = "master"
		}
		if !strings.Contains(*e.Ref, ref) {
			continue
		}
		if _, ex := fns[v.Path]; ex {
			find = true
			cfs = append(cfs, v)
		}
	}
	return cfs, find
}

//getOwnersChangeFile Get the changed owner configuration file,
// the returned slice  is the sig group name or changed file path
// and the value is the file configuration object
func getOwnersChangeFile(e *sdk.PushEvent, files []cfgFilePath) ([]string, bool) {
	fns := getAllChangeFiles(e)
	if len(fns) == 0 {
		return nil, false
	}
	rcm := make([]string, 0)
	find := false
	for _, v := range files {
		if e.Repository.Namespace != v.Owner || e.Repository.Name != v.Repo || e.Ref == nil {
			continue
		}
		ref := v.Ref
		if ref == "" {
			ref = "master"
		}
		if !strings.Contains(*e.Ref, ref) || v.Path == "" {
			continue
		}
		idx := strings.Index(v.Path, judgePathContain)
		if idx != -1 {
			sIdx := idx + len(ownerCfgPathSplit)
			preStr := v.Path[0:idx]
			sufStr := v.Path[sIdx:]
			for pk := range fns {
				if len(pk) > 0 && strings.HasPrefix(pk, preStr) && strings.HasSuffix(pk, sufStr) {
					ed := len(pk) - len(sufStr)
					sn := pk[idx:ed]
					rcm = append(rcm, sn)
					find = true
					break
				}
			}
		} else {
			for pk := range fns {
				if len(pk) > 0 && pk == v.Path {
					rcm = append(rcm, pk)
					find = true
					break
				}
			}
		}
	}
	return rcm, find
}

func getAllChangeFiles(e *sdk.PushEvent) map[string]struct{} {
	fns := make(map[string]struct{})
	for _, v := range e.Commits {
		if len(v.Added) > 0 {
			for _, fn := range v.Added {
				fns[fn] = struct{}{}
			}
		}
		if len(v.Modified) > 0 {
			for _, fn := range v.Modified {
				fns[fn] = struct{}{}
			}
		}
	}
	return fns
}

func getOwnerFileCfgBySigName(ofs []cfgFilePath, sn string) []cfgFilePath {
	var rf []cfgFilePath
	if sn == "" {
		return rf
	}
	sc := sn + "/"
	for _, v := range ofs {
		if strings.Contains(v.Path, judgePathContain) {
			v.Path = strings.Replace(v.Path, "*", sn, -1)
			rf = append(rf, v)
			continue
		} else if strings.Contains(v.Path, sc) {
			rf = append(rf, v)
		}
	}
	return rf
}

func getLegalCfgFile(fps map[string]struct{}, cfs []cfgFilePath) []cfgFilePath {
	lfs := make([]cfgFilePath, 0, len(cfs))
	if len(fps) == 0 || len(cfs) == 0 {
		return lfs
	}
	mr := make(map[int]struct{})
	for i, v := range cfs {
		for k := range fps {
			fp := ""
			if strings.Contains(k, "/") {
				fp = v.Owner + "/" + v.Repo
			} else {
				fp = v.Owner
			}
			if fp == k {
				mr[i] = fps[k]
			}
		}
	}
	for k := range mr {
		lfs = append(lfs, cfs[k])
	}
	return lfs
}
