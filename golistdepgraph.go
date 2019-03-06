// 递归使用 go list -json
// 收集引用关系
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type DepContext struct {
	Ignored         map[string]bool
	IgnoredPrefixes []string
	IgnoredKeyWords []string
	IgnoreStdlib    bool
	DelveGoroot     bool
	TagList         string
	IncludeTests    bool
	BuildTags       []string
	MaxLevel        int
}

func main() {
	var (
		ignoreStdlib   = flag.Bool("s", false, "ignore packages in the Go standard library")
		delveGoroot    = flag.Bool("d", false, "show dependencies of packages in the Go standard library")
		ignorePrefixes = flag.String("p", "", "a comma-separated list of prefixes to ignore")
		ignorePackages = flag.String("i", "", "a comma-separated list of packages to ignore")
		ignoreKeywords = flag.String("k", "", "a comma-separated list of keywords to ignore")
		tagList        = flag.String("tags", "", "a comma-separated list of build tags to consider satisified during the build")
		includeTests   = flag.Bool("t", false, "include test packages")
		maxLevel       = flag.Int("l", 256, "max level of go dependency graph")
	)

	flag.Parse()
	args := flag.Args()
	depContext := DepContext{
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

	pkgMap := make(map[string]JsonObject)
	err = WalkDepGraph(path, pkgName, depContext, pkgMap, 0)
	if err != nil {
		log.Fatal(err)
	}

	dotOutput(pkgMap, depContext, os.Stdout)
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
	return d.Ignored[importPath] ||
		(pkg.GetBool("Goroot") && d.IgnoreStdlib) ||
		hasPrefixes(importPath, d.IgnoredPrefixes) ||
		hasKeywords(importPath, d.IgnoredKeyWords)
}

type IdState struct {
	nextId int
	ids    map[string]int
}

func (idState *IdState) getId(name string) int {
	id, ok := idState.ids[name]
	if !ok {
		id = idState.nextId
		idState.nextId++
		idState.ids[name] = id
	}
	return id
}

func dotOutput(pkgs map[string]JsonObject, dc DepContext, out io.Writer) {
	//importMap := make(map[string]int)
	//for index, pkg := range pkgs {
	//	importMap[pkg.GetString("ImportPath")] = index
	//}
	importPaths := []string{}
	for importPath, _ := range pkgs {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)
	idState := &IdState{ids: make(map[string]int)}

	fmt.Fprintln(out, "digraph G {")
	for _, importPath := range importPaths {
		pkg := pkgs[importPath]
		pkgId := idState.getId(importPath)

		if dc.IsIgnored(pkg) {
			continue
		}

		var color string
		if pkg.GetBool("Goroot") {
			color = "palegreen"
		} else if len(pkg.GetStringSlice("CgoFiles")) > 0 {
			color = "darkgoldenrod1"
		} else {
			color = "paleturquoise"
		}
		var fontColor string
		if pkg.GetBool("Incomplete") {
			fontColor = "red"
		} else if pkg.GetBool("Stale") {
			fontColor = "blue"
		} else {
			fontColor = "black"
		}
		// what about pkg.GetBool("Incomplete"), pkg.GetBool("Stale"), pkg.GetString("StaleReason")?

		fmt.Fprintf(out, "_%d [label=\"%s\" style=\"filled\" color=\"%s\" fontcolor=\"%s\"];\n", pkgId, pkg.GetString("ImportPath"), color, fontColor)

		if pkg.GetBool("Goroot") && !dc.DelveGoroot {
			continue
		}

		for _, imp := range pkg.GetStringSlice("Imports") {
			impPkg, ok := pkgs[imp]
			if !ok || dc.IsIgnored(impPkg) {
				continue
			}
			impId := idState.getId(imp)
			fmt.Fprintf(out, "_%d -> _%d;\n", pkgId, impId)
		}
	}
	fmt.Fprintln(out, "}")
}

func JsonImmediateDep(path string, pkgName string) (JsonObject, error) {
	pkgSrc := pkgName
	cmd := exec.Command("go", "list", "-e", "-json", pkgSrc)
	cmd.Dir = path
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return NewJsonObject(), err
	}
	errStr := string(stderr.Bytes())
	if len(errStr) > 0 {
		fmt.Printf("stderr contained: %s\n", errStr)
	}
	js := NewJsonSeq(stdout.Bytes())
	return js[0], nil
}

type JsonObject map[string]interface{}

func NewJsonObject() JsonObject {
	tmp := make(map[string]interface{})
	return tmp
}

func (jo JsonObject) String() string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.Encode(jo)
	return buf.String()
}

func (jo JsonObject) GetString(key string) string {
	i, ok := jo[key]
	if !ok {
		return ""
	}
	switch v := i.(type) {
	case string:
		return v
	}

	return ""
}

func (jo JsonObject) GetStringSlice(key string) []string {
	i, ok := jo[key]
	if !ok {
		return []string{}
	}
	switch v := i.(type) {
	case []interface{}:
		a := make([]string, len(v))
		for index, elem := range v {
			switch e := elem.(type) {
			case string:
				a[index] = e
			default:
				a[index] = ""
			}
		}
		return a
	}

	return []string{}
}

func (jo JsonObject) GetBool(key string) bool {
	i, ok := jo[key]
	if !ok {
		return false
	}
	switch v := i.(type) {
	case bool:
		return v
	}

	return false
}

type JsonSeq []JsonObject

func NewJsonSeq(buf []byte) JsonSeq {
	seq := []JsonObject{}
	reader := bytes.NewReader(buf)
	decoder := json.NewDecoder(reader)
	var err error
	for err == nil {
		jo := NewJsonObject()
		err = decoder.Decode(&jo)
		seq = append(seq, jo)
	}
	if len(seq) == 0 {
		return seq
	} else {
		return seq[:len(seq)-1]
	}
}

func (js JsonSeq) String() string {
	var b strings.Builder
	fmt.Fprintln(&b, "[")
	for i, jo := range js {
		if i != 0 {
			fmt.Fprintln(&b, ",")
		}
		fmt.Fprint(&b, jo.String())
	}
	fmt.Fprintln(&b, "]")
	return b.String()
}

// analogous to godepgraph's processPackage

func WalkDepGraph(dir string, pkgImportPath string, dc DepContext, pkgMap map[string]JsonObject, level int) error {
	if level++; level > dc.MaxLevel {
		return nil
	}

	if dc.Ignored[pkgImportPath] {
		return nil
	}

	pkg, err := JsonImmediateDep(dir, pkgImportPath)
	if err != nil {
		return fmt.Errorf("failed to import %s, %s", pkgImportPath, err)
	}

	if dc.IsIgnored(pkg) {
		return nil
	}

	pkgMap[pkg.GetString("ImportPath")] = pkg

	if (pkg.GetBool("Goroot")) && !dc.DelveGoroot {
		return nil
	}

	for _, imp := range getImports(dc, pkg) {
		if _, ok := pkgMap[imp]; !ok {
			if err := WalkDepGraph(dir, imp, dc, pkgMap, level); err != nil {
				return err
			}
		}
	}
	return nil
}

func getImports(dc DepContext, pkg JsonObject) []string {
	allImports := pkg.GetStringSlice("Imports")
	if dc.IncludeTests {
		allImports = append(allImports, pkg.GetStringSlice("TestImports")...)
		allImports = append(allImports, pkg.GetStringSlice("XTestImports")...)
	}
	var imports []string
	found := make(map[string]struct{})
	pkgImportPath := pkg.GetString("ImportPath")
	for _, imp := range allImports {
		if imp == pkgImportPath {
			// avoiding self-reference
			continue
		}
		if _, ok := found[imp]; ok {
			// skipping repeated packges contained in allImports
			continue
		}
		found[imp] = struct{}{}
		imports = append(imports, imp)
	}
	return imports
}
