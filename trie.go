package main

import "strings"

type Trie struct {
	children map[string]*Trie
	matched  bool
	extra    string
}

func NewTrie() *Trie {
	return &Trie{children: map[string]*Trie{}}
}

func (t *Trie) Insert(domain string, extra ...string) {
	parts := strings.Split(domain, ".")
	node := t
	for i := len(parts) - 1; i >= 0; i-- {
		if node.children[parts[i]] == nil {
			node.children[parts[i]] = &Trie{children: map[string]*Trie{}}
		}
		node = node.children[parts[i]]
	}
	node.matched = true
	if len(extra) >= 1 {
		node.extra = extra[0]
	}
}

func (t *Trie) Match(domain string) bool {
	parts := strings.Split(domain, ".")
	node := t
	for i := len(parts) - 1; i >= 0; i-- {
		if node.matched {
			return true
		}
		next, ok := node.children[parts[i]]
		if !ok {
			return false
		}
		node = next
	}
	return node.matched
}

func (t *Trie) MatchEx(domain string) (string, bool) {
	parts := strings.Split(domain, ".")
	node := t
	for i := len(parts) - 1; i >= 0; i-- {
		if node.matched {
			return t.extra, true
		}
		next, ok := node.children[parts[i]]
		if !ok {
			return t.extra, false
		}
		node = next
	}
	return t.extra, node.matched
}
