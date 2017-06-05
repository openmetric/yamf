package utils

import (
	"regexp"
)

var regexpCache map[string]*regexp.Regexp = make(map[string]*regexp.Regexp)

// CacheGetRegexp compiles a pattern and cache the result for future callings
func CacheGetRegexp(p string) (*regexp.Regexp, error) {
	if r, ok := regexpCache[p]; ok {
		return r, nil
	}

	if r, err := regexp.Compile(p); err != nil {
		return nil, err
	} else {
		regexpCache[p] = r
		return r, nil
	}
}
