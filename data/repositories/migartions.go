package repositories

import (
	"time"
	//	"fmt"
	//	"time"

	"github.com/graph-uk/combat-server/data"
	"github.com/graph-uk/combat-server/data/models"
	"github.com/jinzhu/gorm"
)

// Migrations is repsoitory creates db schema
type Migrations struct {
	context data.Context
}

// migration for Configs table, contains notification-disabling etc...
func (t *Migrations) migrateConfig() error {
	//defaultConfig = &models.Config{1,time.Now(), false}

	dbConfig := &models.Config{}
	query := func(db *gorm.DB) {
		db.First(dbConfig, nil)
	}
	err := t.context.Execute(query)
	if err != nil {
		return err
	}

	// if config not found, or first recordID ==0
	if dbConfig.ID == 0 {
		// clear table ""
		query = func(db *gorm.DB) {
			db.Delete(&models.Config{}, `id = *`)
		}
		err = t.context.Execute(query)
		if err != nil {
			return err
		}

		//insert default config.
		query = func(db *gorm.DB) {
			db.Save(&models.Config{1, time.Now(), true})
		}
		err = t.context.Execute(query)
		if err != nil {
			return err
		}
	}
	return err
}

//Apply migrations to the repository
func (t *Migrations) Apply() error {
	query := func(db *gorm.DB) {
		db.AutoMigrate(&models.Case{}, &models.Session{}, &models.Try{}, &models.Config{})
	}
	err := t.context.Execute(query)
	if err != nil {
		return err
	}

	return t.migrateConfig()
}
