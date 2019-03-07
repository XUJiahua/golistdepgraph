package core

import "strings"

type DepContext struct {
	Ignored         map[string]bool
	IgnoredPrefixes []string
	IgnoredKeyWords []string
	IgnoreStdlib    bool
	OnlyPrefixes    []string
	DelveGoroot     bool
	TagList         string
	IncludeTests    bool
	BuildTags       []string
	MaxLevel        int
	TrimPrefix      string
}

func hasPrefixes(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func hasKeywords(s string, keywords []string) bool {
	for _, p := range keywords {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func (d DepContext) IsIgnored(pkg JsonObject) bool {
	importPath := pkg.GetString("ImportPath")

	if len(d.OnlyPrefixes) > 0 && !hasPrefixes(importPath, d.OnlyPrefixes) {
		return true
	}

	return d.Ignored[importPath] ||
		(pkg.GetBool("Goroot") && d.IgnoreStdlib) ||
		hasPrefixes(importPath, d.IgnoredPrefixes) ||
		hasKeywords(importPath, d.IgnoredKeyWords)
}
