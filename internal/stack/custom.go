package stack

import "fmt"

type CustomPatches []struct {
	Repo    string
	Patches []string
	Branch  string
}

type CustomScripts []struct {
	Repo    string
	Scripts []string
	Branch  string
}

type CustomPrebuilts []struct {
	Repo    string
	Modules []string
}

type CustomManifestRemotes []struct {
	Name     string
	Fetch    string
	Revision string
}

func (c CustomManifestRemotes) String() string {
	var output string
	for _, manifest := range c {
		output += fmt.Sprintf(`<remote name="%v" fetch="%v" revision="%v" />\n`, manifest.Name, manifest.Fetch, manifest.Revision)
	}
	return output
}

type CustomManifestProjects []struct {
	Path    string
	Name    string
	Remote  string
	Modules []string
}

func (c CustomManifestProjects) String() string {
	var output string
	for _, p := range c {
		output += fmt.Sprintf(`<project path="%v" name="%v" remote="%v" />\n`, p.Path, p.Name, p.Remote)
	}
	return output
}