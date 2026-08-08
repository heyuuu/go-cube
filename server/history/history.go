package history

import (
	"gorm.io/gorm"
)

type ProjectSelectLog struct {
	gorm.Model
	Project string `gorm:"project"`
	Alfred  bool   `gorm:"alfred"`
}

type ProjectOpenLog struct {
	gorm.Model
	Project string `gorm:"project"`
	Opener  string `gorm:"opener"`
	Alfred  bool   `gorm:"alfred"`
}
