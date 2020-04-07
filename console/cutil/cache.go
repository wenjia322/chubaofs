package cutil

import (
	"fmt"
	"github.com/patrickmn/go-cache"
)

var userTimeout = Duration(20)

var userCache = cache.New(cache.NoExpiration, cache.NoExpiration)

func TokenValidate(token string) (interface{}, error) {
	v, found := userCache.Get(token)

	if !found {
		return nil, fmt.Errorf("the token:[%s] is invalidate", token)
	}

	userCache.Set()
}
