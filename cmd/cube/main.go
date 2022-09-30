package main

import (
	"fmt"
	"go-cube/internal/project"
	"sort"
)

func main() {
	manager := project.DefaultManager()

	projects := manager.Projects()

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	for index, proj := range projects {
		fmt.Printf("[%3d] %s %s\n", index, proj.Name, proj.Path)
	}
}
