package repohandle

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
)

var (
	instance *cacheProcessedFile
	once     sync.Once
)

type cacheProcessedFile struct {
	filePath string
	sync.Mutex
}

func newCache(filePath string) *cacheProcessedFile {
	once.Do(func() {
		instance = &cacheProcessedFile{filePath: filePath}
	})
	return instance
}

func (c *cacheProcessedFile) cacheInit() error {
	if exist(c.filePath) {
		return nil
	}
	file, err := os.Create(c.filePath)
	if err != nil {
		return err
	}
	err = file.Close()
	return err
}

func (c *cacheProcessedFile) loadCache() ([]cfgFilePath, error) {
	c.Lock()
	defer c.Unlock()
	var cacheRepos []cfgFilePath
	data, err := ioutil.ReadFile(c.filePath)
	if err != nil {
		return cacheRepos, err
	}
	err = json.Unmarshal(data, &cacheRepos)
	return cacheRepos, err
}

func (c *cacheProcessedFile) saveCache(repoFiles []cfgFilePath) error {
	data, err := json.Marshal(&repoFiles)
	if err != nil {
		return err
	}
	c.Lock()
	defer c.Unlock()
	file, err := os.OpenFile(c.filePath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func (c *cacheProcessedFile) emptyCache() error {
	rf := make([]cfgFilePath, 0)
	return c.saveCache(rf)
}

func exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}
