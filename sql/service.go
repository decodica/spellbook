package sql

import (
	"context"
	"github.com/jinzhu/gorm"
)

const name = "__sql_service"

type key string

const sqlKey key = "__sql_connection"

type Service struct {
	Connection string
	Debug bool
	Migration Migration
	db *gorm.DB
}

type Migration interface {
	Execute(db *gorm.DB)
}

func (service *Service) Name() string {
	return name
}

func (service *Service) Initialize() {
	db, err := gorm.Open("postgres", service.Connection)
	if err != nil {
		panic(err)
	}

	db.LogMode(service.Debug)


	service.db = db
	if service.Migration != nil {
		service.Migration.Execute(service.db)
	}
}

// adds the appengine client to the context
func (service *Service) OnStart(ctx context.Context) context.Context {
	db := service.db.New()
	return context.WithValue(ctx, sqlKey, db)
}

func (service *Service) OnEnd(ctx context.Context) {}

func (service *Service) Destroy() {
	if service.db != nil {
		service.db.Close()
	}
}

func FromContext(ctx context.Context) *gorm.DB {
	if bundle := ctx.Value(sqlKey); bundle != nil {
		return bundle.(*gorm.DB)
	}
	return nil
}

func ToColumnName(name string) string {
	return gorm.ToColumnName(name)
}
