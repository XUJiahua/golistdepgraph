package main

import (
	"flag"
	"github.com/XUJiahua/golistdepgraph/core"
	"github.com/XUJiahua/golistdepgraph/render"
	"log"
	"os"
	"strings"
)

func main() {
	var (
		ignoreStdlib   = flag.Bool("s", false, "ignore packages in the Go standard library")
		delveGoroot    = flag.Bool("d", false, "show dependencies of packages in the Go standard library")
		ignorePrefixes = flag.String("p", "", "a comma-separated list of prefixes to ignore")
		ignorePackages = flag.String("i", "", "a comma-separated list of packages to ignore")
		ignoreKeywords = flag.String("k", "", "a comma-separated list of keywords to ignore")
		onlyPrefix     = flag.String("o", "", "a comma-separated list of prefixes to include")
		tagList        = flag.String("tags", "", "a comma-separated list of build tags to consider satisified during the build")
		includeTests   = flag.Bool("t", false, "include test packages")
		maxLevel       = flag.Int("l", 256, "max level of go dependency graph")
	)

	flag.Parse()
	args := flag.Args()
	depContext := core.DepContext{
		IgnoreStdlib: *ignoreStdlib,
		DelveGoroot:  *delveGoroot,
		IncludeTests: *includeTests,
		MaxLevel:     *maxLevel,
		Ignored:      make(map[string]bool)}

	if *ignorePrefixes != "" {
		depContext.IgnoredPrefixes = strings.Split(*ignorePrefixes, ",")
	}
	if *ignoreKeywords != "" {
		depContext.IgnoredKeyWords = strings.Split(*ignoreKeywords, ",")
	}
	if *ignorePackages != "" {
		for _, p := range strings.Split(*ignorePackages, ",") {
			depContext.Ignored[p] = true
		}
	}
	if *onlyPrefix != "" {
		depContext.OnlyPrefixes = strings.Split(*onlyPrefix, ",")
	}
	if *tagList != "" {
		depContext.BuildTags = strings.Split(*tagList, ",")
	}

	if len(args) != 1 {
		log.Fatal("need one package name to process")
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get cwd: %s", err)
	}
	path := cwd
	pkgName := args[0]

	pkgMap := make(map[string]core.JsonObject)
	// get pkgMap
	err = core.WalkDepGraph(path, pkgName, depContext, pkgMap, 0)
	if err != nil {
		log.Fatal(err)
	}

	// render to dot format
	render.DotOutput(pkgMap, depContext, os.Stdout)
}
