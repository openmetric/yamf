package types

import (
	"regexp"
	"sync"
)

// provides cached regexp compile result

var regexpCache = struct {
	cache map[string]*regexp.Regexp
	sync.RWMutex
}{
	cache: make(map[string]*regexp.Regexp),
}

func RegexpMustCompile(str string) *regexp.Regexp {
	regexpCache.Lock()
	defer regexpCache.Unlock()

	if r, ok := regexpCache.cache[str]; ok {
		return r.Copy()
	}

	r := regexp.MustCompile(str)
	regexpCache.cache[str] = r
	return r.Copy()
}

func RegexpCompile(str string) (*regexp.Regexp, error) {
	regexpCache.Lock()
	defer regexpCache.Unlock()

	if r, ok := regexpCache.cache[str]; ok {
		return r.Copy(), nil
	}

	if r, err := regexp.Compile(str); err == nil {
		regexpCache.cache[str] = r
		return r.Copy(), err
	} else {
		return nil, err
	}
}
