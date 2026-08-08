package history

import (
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db: db,
	}
}

func (s *Service) AddProjectSelectLog(project string, alfred bool) error {
	s.db.Create(&ProjectSelectLog{
		Project: project,
		Alfred:  alfred,
	})
	return nil
}

func (s *Service) LeastSelectedProjects(limit int, alfred bool) []string {
	var projects []string
	s.db.Model(&ProjectSelectLog{}).
		Select("project").
		Where(&ProjectSelectLog{
			Alfred: alfred,
		}).
		Group("project").
		Order("max(id) desc").
		Limit(limit).
		Find(&projects)

	return projects
}

func (s *Service) AddProjectOpenLog(project string, opener string, alfred bool) error {
	s.db.Create(&ProjectOpenLog{
		Project: project,
		Opener:  opener,
		Alfred:  alfred,
	})
	return nil
}

func (s *Service) LeastProjectOpenApps(project string, limit int, alfred bool) []string {
	var projects []string
	s.db.Model(&ProjectOpenLog{}).
		Select("opener").
		Where(&ProjectOpenLog{
			Project: project,
			Alfred:  alfred,
		}).
		Group("opener").
		Order("max(id) desc").
		Limit(limit).
		Find(&projects)

	return projects
}
