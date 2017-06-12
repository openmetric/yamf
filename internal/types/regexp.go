package types

import (
	"regexp"
)

// provides cached regexp compile result

var regexpCache = NewGenericCache(
	func(str interface{}) (interface{}, error) {
		return regexp.Compile(str.(string))
	},
)

func RegexpMustCompile(str string) *regexp.Regexp {
	if r, err := regexpCache.GetOrCreate(str); err != nil {
		panic(err)
	} else {
		return r.(*regexp.Regexp).Copy()
	}
}

func RegexpCompile(str string) (*regexp.Regexp, error) {
	if r, err := regexpCache.GetOrCreate(str); err != nil {
		return nil, err
	} else {
		return r.(*regexp.Regexp).Copy(), err
	}
}
