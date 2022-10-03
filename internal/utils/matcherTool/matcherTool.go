package matcherTool

import (
	"go-cube/internal/matcher"
	"go-cube/internal/project"
)

func StringMatcher(strings []string) *matcher.Matcher[string] {
	return matcher.NewStringMatcher(strings, matcher.DefaultScorer)
}

func ProjectMatcher(projects []project.Project) *matcher.Matcher[project.Project] {
	return matcher.NewKeywordMatcher(projects, func(proj project.Project) string { return proj.Name }, matcher.DefaultScorer)
}

func WorkspaceMatcher(workspaces []project.Workspace) *matcher.Matcher[project.Workspace] {
	return matcher.NewKeywordMatcher(workspaces, func(workspace project.Workspace) string { return workspace.Name() }, matcher.DefaultScorer)
}
