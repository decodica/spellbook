package sql

import (
	"context"
	"decodica.com/spellbook"
	"fmt"
	"github.com/jinzhu/gorm"
	"strings"
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

func OperatorToSymbol(op spellbook.FilterOperator) string {
	switch op {
	case spellbook.FilterOperatorLessThan:
		return "<"
	case spellbook.FilterOperatorGreaterThan:
		return ">"
	case spellbook.FilterOperatorLessOrEqualThan:
		return "<="
	case spellbook.FilterOperatorGreaterOrEqualThan:
		return ">="
	case spellbook.FilterOperatorExact:
		return "="
	case spellbook.FilterOperatorLike:
		return "LIKE"
	}
	return ""
}

func FiltersToCondition(fs []spellbook.Filter) string {
	if fs == nil || len(fs) == 0 {
		return ""
	}
	where := strings.Builder{}
	for i, f := range fs {
		if i > 0 {
			where.WriteString(" AND ")
		}
		os := OperatorToSymbol(f.Operator)
		dbField := ToColumnName(f.Field)
		val := f.Value
		if val == "null" && f.Operator == spellbook.FilterOperatorExact {
			where.WriteString(fmt.Sprintf("%q IS NULL", dbField))
		} else {
			where.WriteString(fmt.Sprintf("%q %s %q", dbField, os, f.Value))
		}
	}
	return where.String()
}
