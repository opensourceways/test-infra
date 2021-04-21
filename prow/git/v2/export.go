package git

import (
	"os"
	"path"
)

func NewClientFactoryOnPV(opt ClientFactoryOpt) (ClientFactory, error) {
	option := &ClientFactoryOpts{}
	opt(option)

	cacheDir := path.Join(*option.CacheDirBase, option.Host, "gitcache")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		if err := os.Mkdir(cacheDir, 0732); err != nil {
			return nil, err
		}
	}

	f, err := NewClientFactory(opt)
	if err != nil {
		return nil, err
	}

	c := f.(*clientFactory)
	c.cacheDir = cacheDir
	return c, nil
}
