package converter

import (
	"github.com/heyuuu/go-cube/internal/dto/response"
	"github.com/heyuuu/go-cube/internal/entities"
)

func ToProjectResponseDto(entity *entities.Project) response.ProjectDto {
	return response.ProjectDto{
		Name:    entity.Name(),
		Path:    entity.Path(),
		RepoUrl: entity.RepoUrl(),
		Tags:    entity.Tags(),
	}
}

func ToWorkspaceResponseDto(entity *entities.Workspace) response.WorkspaceDto {
	return response.WorkspaceDto{
		Name:       entity.Name(),
		Root:       entity.Path(),
		PreferApps: entity.PreferApps(),
		Scanner:    toScannerResponseData(entity.Scanner()),
	}
}

func toScannerResponseData(scanner entities.ProjectScanner) map[string]any {
	switch sc := scanner.(type) {
	case *entities.GitProjectScanner:
		return map[string]any{
			"type":     "git",
			"maxDepth": sc.MaxDepth(),
		}
	default:
		return nil
	}
}

func ToApplicationResponseDto(entity *entities.Application) response.ApplicationDto {
	return response.ApplicationDto{
		Name: entity.Name(),
		Bin:  entity.Bin(),
	}
}

func ToRemoteResponseDto(entity *entities.Remote) response.RemoteDto {
	return response.RemoteDto{
		Name:        entity.Name(),
		Host:        entity.Host(),
		DefaultPath: entity.DefaultPath(),
	}
}
