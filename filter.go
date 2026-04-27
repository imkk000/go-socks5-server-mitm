package main

import "sync"

var blockList = NewTrie()

func isBlock(name string) bool {
	return blockList.Match(name)
}

var (
	// TODO: evict
	proxyMapIP = new(sync.Map)
	proxyList  = NewTrie()
)

func isProxy(name string) bool {
	val, found := proxyMapIP.Load(name)
	if found {
		name = val.(string)
	}
	return proxyList.Match(name)
}

var skipProxy = NewTrie()

func isSkipProxy(name string) bool {
	return skipProxy.Match(name)
}
