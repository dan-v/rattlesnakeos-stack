package stack

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

type CustomManifestProjects []struct {
	Path    string
	Name    string
	Remote  string
	Modules []string
}
