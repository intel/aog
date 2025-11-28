//*****************************************************************************
// Copyright 2024-2025 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//*****************************************************************************

package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/provider/template"
	"github.com/intel/aog/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var cstZone = time.FixedZone("CST", 8*3600)

// LocalTimePlugin is a GORM plugin for automatically setting LocalTime type timestamp fields
type LocalTimePlugin struct{}

// Name returns the plugin name
func (p *LocalTimePlugin) Name() string {
	return "localtime"
}

// Initialize initializes the plugin and registers callbacks
func (p *LocalTimePlugin) Initialize(db *gorm.DB) error {
	// Register callback for setting timestamps on create
	if err := db.Callback().Create().Before("gorm:create").Register("localtime:set_create_time", setCreateTime); err != nil {
		return err
	}

	// Register callback for setting timestamps on update
	if err := db.Callback().Update().Before("gorm:update").Register("localtime:set_update_time", setUpdateTime); err != nil {
		return err
	}

	return nil
}

// setCreateTime sets CreatedAt and UpdatedAt fields before creating records
func setCreateTime(db *gorm.DB) {
	if db.Statement.Schema != nil {
		setTimestampFields(db, "CreatedAt", "UpdatedAt")
	}
}

// setUpdateTime sets UpdatedAt field before updating records
func setUpdateTime(db *gorm.DB) {
	if db.Statement.Schema != nil {
		setTimestampFields(db, "UpdatedAt")
	}
}

// setTimestampFields sets the specified timestamp fields
func setTimestampFields(db *gorm.DB, fieldNames ...string) {
	now := time.Now().In(cstZone)
	rv := db.Statement.ReflectValue

	// Handle both single objects and batch objects uniformly
	forEachStruct(rv, func(structValue reflect.Value) {
		setFieldsOnValue(db, structValue, now, fieldNames)
	})
}

// forEachStruct iterates over reflection values and executes callback for each struct
// Supports single structs, pointers, slices and arrays
func forEachStruct(rv reflect.Value, fn func(reflect.Value)) {
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		// Batch operation: iterate through each element in the slice
		for i := 0; i < rv.Len(); i++ {
			if structValue := unwrapToStruct(rv.Index(i)); structValue.IsValid() {
				fn(structValue)
			}
		}
	default:
		// Single operation: process directly
		if structValue := unwrapToStruct(rv); structValue.IsValid() {
			fn(structValue)
		}
	}
}

// unwrapToStruct unwraps a reflection value to a struct
// Handles pointers, multiple levels of pointers, etc.
func unwrapToStruct(v reflect.Value) reflect.Value {
	// Dereference pointers
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}

	// Verify if it's a valid struct
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	return v
}

// setFieldsOnValue sets timestamp fields on the given struct value
func setFieldsOnValue(db *gorm.DB, structValue reflect.Value, now time.Time, fieldNames []string) {
	if !structValue.IsValid() {
		return
	}

	for _, field := range db.Statement.Schema.Fields {
		for _, name := range fieldNames {
			if field.Name == name {
				_ = field.Set(db.Statement.Context, structValue, now)
				break
			}
		}
	}
}

// SQLite implements the Datastore interface
type SQLite struct {
	db *gorm.DB
}

// New creates a new SQLite instance
func New(dbPath string) (*SQLite, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		file, err := os.Create(dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create database file: %v", err)
		}
		err = file.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to close database file: %v", err)
		}
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().In(cstZone)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Use LocalTimePlugin to automatically set timestamps
	if err := db.Use(&LocalTimePlugin{}); err != nil {
		return nil, fmt.Errorf("failed to register LocalTimePlugin: %v", err)
	}

	return &SQLite{db: db}, nil
}

// migrateProtocolField intelligently sets Protocol field for existing records without it
func (ds *SQLite) migrateProtocolField() error {
	var providers []types.ServiceProvider
	if err := ds.db.Find(&providers).Error; err != nil {
		return err
	}

	for i := range providers {
		p := &providers[i]
		// Only update records without Protocol set
		if p.Protocol == "" {
			// Intelligent inference based on URL and service characteristics
			if strings.Contains(p.URL, ":9000") {
				// OpenVINO gRPC services on port 9000
				if strings.Contains(p.ServiceName, "ws") || strings.Contains(p.ServiceName, "speech-to-text-ws") {
					p.Protocol = types.ProtocolGRPC_STREAM
				} else {
					p.Protocol = types.ProtocolGRPC
				}
			} else if strings.HasPrefix(p.URL, "wss://") {
				// WebSocket services
				p.Protocol = types.ProtocolHTTP // WebSocket is handled as HTTP by HTTPInvoker
			} else {
				// Default to HTTP for all other cases
				p.Protocol = types.ProtocolHTTP
			}

			// Save the updated record
			if err := ds.db.Save(p).Error; err != nil {
				return fmt.Errorf("failed to update protocol for provider %s: %v", p.ProviderName, err)
			}
		}
	}

	return nil
}

// Init TODO need to consider table structure changes here
func (ds *SQLite) Init() error {
	// Auto-migrate table structures
	if err := ds.db.AutoMigrate(
		&types.ServiceProvider{},
		&types.Service{},
		&types.Model{},
		&types.VersionUpdateRecord{},
		&types.DataMigrateVersion{},
		// RAG tables
		&types.RagFile{},
		&types.RagChunk{},
	); err != nil {
		return fmt.Errorf("failed to initialize database tables: %v", err)
	}

	// Migrate Protocol field for existing records
	if err := ds.migrateProtocolField(); err != nil {
		return fmt.Errorf("failed to migrate protocol field: %v", err)
	}

	if err := ds.insertInitialData(); err != nil {
		return fmt.Errorf("failed to insert initial data: %v", err)
	}

	return nil
}

// insertInitialData inserts initialization data
func (ds *SQLite) insertInitialData() error {
	// service
	initService := make([]*types.Service, 0)
	initService = append(initService, &types.Service{
		Name:         "chat",
		HybridPolicy: "default",
		Status:       -1,
		CanInstall:   1,
		Avatar:       types.ServiceChatAvatar,
	}, &types.Service{
		Name:         "models",
		HybridPolicy: "default",
		Status:       -1,
		CanInstall:   0,
	}, &types.Service{
		Name:         "embed",
		HybridPolicy: "default",
		Status:       -1,
		CanInstall:   1,
		Avatar:       types.ServiceEmbedAvatar,
	}, &types.Service{
		Name:         "rerank",
		HybridPolicy: "default",
		Status:       -1,
		CanInstall:   1,
		Avatar:       types.ServiceRerankAvatar,
	}, &types.Service{
		Name:         "generate",
		HybridPolicy: "default",
		Status:       -1,
		CanInstall:   1,
		Avatar:       types.ServiceGenerateAvatar,
	}, &types.Service{
		Name:         "text-to-image",
		HybridPolicy: "always_remote",
		Status:       -1,
		CanInstall:   1,
		Avatar:       types.ServiceTextToImageAvatar,
	}, &types.Service{
		Name:         "speech-to-text",
		HybridPolicy: "default",
		Status:       -1,
		CanInstall:   1,
		Avatar:       types.ServiceSpeechToTextAvatar,
	}, &types.Service{
		Name:         "speech-to-text-ws",
		HybridPolicy: "default",
		Status:       -1,
		CanInstall:   1,
		Avatar:       types.ServiceSpeechToTextWSAvatar,
	}, &types.Service{
		Name:         "text-to-speech",
		HybridPolicy: "default",
		Status:       -1,
		CanInstall:   1,
		Avatar:       types.ServiceTextToSpeechAvatar,
	}, &types.Service{
		Name:         "image-to-video",
		HybridPolicy: "default",
		Status:       -1,
		CanInstall:   1,
		Avatar:       types.ServiceImageToVideoAvatar,
	}, &types.Service{
		Name:         "image-to-image",
		HybridPolicy: "default",
		Status:       -1,
		CanInstall:   1,
		Avatar:       types.ServiceImageToImageAvatar,
	})

	needInitService := make([]*types.Service, 0)
	for _, service := range initService {
		err := ds.Get(context.Background(), service)
		if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
			return fmt.Errorf("failed to create initial service: %v", err)
		} else if errors.Is(err, datastore.ErrEntityInvalid) {
			needInitService = append(needInitService, service)
		}
	}
	if err := ds.db.CreateInBatches(needInitService, len(needInitService)).Error; err != nil {
		return fmt.Errorf("failed to create initial service: %v", err)
	}
	// service provider
	var serviceProviders []*types.ServiceProvider
	serviceProviderData, err := template.FlavorTemplateFs.ReadFile("service_provider_data.json")
	if err != nil {
		return fmt.Errorf("failed to read service provider data: %v", err)
	}
	if err := json.Unmarshal(serviceProviderData, &serviceProviders); err != nil {
		return fmt.Errorf("failed to unmarshal service provider data: %v", err)
	}
	initServiceProvider := make([]*types.ServiceProvider, 0)
	for _, serviceProvider := range serviceProviders {
		err = ds.Get(context.Background(), serviceProvider)
		if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
			return fmt.Errorf("failed to create initial service: %v", err)
		} else if errors.Is(err, datastore.ErrEntityInvalid) {
			initServiceProvider = append(initServiceProvider, serviceProvider)
		}
	}
	if err := ds.db.CreateInBatches(initServiceProvider, len(initServiceProvider)).Error; err != nil {
		return fmt.Errorf("failed to create initial service provider: %v", err)
	}
	return nil
}

// Add inserts a record
func (ds *SQLite) Add(ctx context.Context, entity datastore.Entity) error {
	if entity == nil {
		return datastore.ErrNilEntity
	}
	if entity.PrimaryKey() == "" {
		return datastore.ErrPrimaryEmpty
	}
	if entity.TableName() == "" {
		return datastore.ErrTableNameEmpty
	}

	// Check if the record already exists
	exist, err := ds.IsExist(ctx, entity)
	if err != nil {
		return err
	}
	if exist {
		return datastore.ErrRecordExist
	}

	if err := ds.db.WithContext(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("failed to insert record: %v", err)
	}
	return nil
}

// BatchAdd inserts multiple records
func (ds *SQLite) BatchAdd(ctx context.Context, entities []datastore.Entity) error {
	if len(entities) == 0 {
		return nil
	}

	return ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, entity := range entities {
			if err := ds.Add(ctx, entity); err != nil {
				return err
			}
		}
		return nil
	})
}

func (ds *SQLite) isZeroValue(v interface{}) bool {
	return v == reflect.Zero(reflect.TypeOf(v)).Interface()
}

// Put updates or inserts a record (upsert operation)
func (ds *SQLite) Put(ctx context.Context, entity datastore.Entity) error {
	if entity == nil {
		return datastore.ErrNilEntity
	}
	if entity.PrimaryKey() == "" {
		return datastore.ErrPrimaryEmpty
	}
	if entity.TableName() == "" {
		return datastore.ErrTableNameEmpty
	}

	// Check if the record exists (based on Index, not primary key)
	exist, err := ds.IsExist(ctx, entity)
	if err != nil {
		return err
	}

	if exist {
		// Update existing record
		fields, values, err := getEntityFieldsAndValues(entity)
		if err != nil {
			return err
		}

		updateMap := make(map[string]interface{})
		primaryKeyFieldName := getPrimaryKeyFieldName(entity)

		for i, field := range fields {
			if field == primaryKeyFieldName {
				continue
			}
			if ds.isZeroValue(values[i]) {
				continue
			}

			updateMap[field] = values[i]
		}

		updateMap["updated_at"] = time.Now().In(cstZone)

		if len(updateMap) == 1 {
			return nil
		}

		db := ds.db.WithContext(ctx).Model(entity)
		for key, value := range entity.Index() {
			db = db.Where(fmt.Sprintf("%s = ?", key), value)
		}

		if err := db.Updates(updateMap).Error; err != nil {
			return fmt.Errorf("failed to update record: %v", err)
		}
	} else {
		// Insert new record
		return ds.Add(ctx, entity)
	}
	return nil
}

func getPrimaryKeyFieldName(entity datastore.Entity) string {
	val := reflect.ValueOf(entity)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		gormTag := field.Tag.Get("gorm")
		if strings.Contains(gormTag, "primaryKey") {
			return field.Name
		}
	}

	return "ID"
}

// Delete removes a record
func (ds *SQLite) Delete(ctx context.Context, entity datastore.Entity) error {
	if entity == nil {
		return datastore.ErrNilEntity
	}
	if entity.PrimaryKey() == "" {
		return datastore.ErrPrimaryEmpty
	}
	if entity.TableName() == "" {
		return datastore.ErrTableNameEmpty
	}

	db := ds.db.WithContext(ctx).Model(entity)
	for key, value := range entity.Index() {
		db = db.Where(fmt.Sprintf("%s = ?", key), value)
	}

	if err := db.Delete(entity).Error; err != nil {
		return fmt.Errorf("failed to delete record: %v", err)
	}
	return nil
}

// Get retrieves a single record
func (ds *SQLite) Get(ctx context.Context, entity datastore.Entity) error {
	if entity == nil {
		return datastore.ErrNilEntity
	}
	if entity.PrimaryKey() == "" {
		return datastore.ErrPrimaryEmpty
	}
	if entity.TableName() == "" {
		return datastore.ErrTableNameEmpty
	}

	db := ds.db.WithContext(ctx).Model(entity)
	for key, value := range entity.Index() {
		db = db.Where(fmt.Sprintf("%s = ?", key), value)
	}

	if err := db.First(entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return datastore.ErrEntityInvalid
		}
		return fmt.Errorf("failed to get record: %v", err)
	}
	return nil
}

// List queries multiple records
func (ds *SQLite) List(ctx context.Context, entity datastore.Entity, options *datastore.ListOptions) ([]datastore.Entity, error) {
	if entity == nil {
		return nil, datastore.ErrNilEntity
	}
	if entity.TableName() == "" {
		return nil, datastore.ErrTableNameEmpty
	}

	db := ds.db.WithContext(ctx).Model(entity)
	for key, value := range entity.Index() {
		db = db.Where(fmt.Sprintf("%s = ?", key), value)
	}

	// Add filter conditions
	if options != nil {
		filters := buildFilterConditions(options.FilterOptions)
		if len(filters) > 0 {
			db = db.Where(strings.Join(filters, " AND "))
		}

		// Add sorting
		if len(options.SortBy) > 0 {
			for _, sort := range options.SortBy {
				order := "ASC"
				if sort.Order == datastore.SortOrderDescending {
					order = "DESC"
				}
				db = db.Order(sort.Key + " " + order)
			}
		}

		// Add pagination
		if options.PageSize > 0 {
			offset := (options.Page - 1) * options.PageSize
			db = db.Limit(options.PageSize).Offset(offset)
		}
	}

	list := make([]datastore.Entity, 0)
	rows, err := db.Rows()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, datastore.ErrRecordNotExist
		}
		return nil, datastore.NewDBError(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		e, err := datastore.NewEntity(entity)
		if err != nil {
			return nil, datastore.ErrEntityInvalid
		}
		if err := ds.db.ScanRows(rows, e); err != nil {
			return nil, datastore.ErrEntityInvalid
		}
		list = append(list, e)
	}
	return list, nil
}

// Count counts the number of records
func (ds *SQLite) Count(ctx context.Context, entity datastore.Entity, options *datastore.FilterOptions) (int64, error) {
	if entity == nil {
		return 0, datastore.ErrNilEntity
	}
	if entity.TableName() == "" {
		return 0, datastore.ErrTableNameEmpty
	}

	db := ds.db.WithContext(ctx).Model(entity)
	for key, value := range entity.Index() {
		db = db.Where(fmt.Sprintf("%s = ?", key), value)
	}

	// Add filter conditions
	if options != nil {
		filters := buildFilterConditions(*options)
		if len(filters) > 0 {
			db = db.Where(strings.Join(filters, " AND "))
		}
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count records: %v", err)
	}
	return count, nil
}

// IsExist checks if a record exists
func (ds *SQLite) IsExist(ctx context.Context, entity datastore.Entity) (bool, error) {
	if entity == nil {
		return false, datastore.ErrNilEntity
	}
	if entity.PrimaryKey() == "" {
		return false, datastore.ErrPrimaryEmpty
	}
	if entity.TableName() == "" {
		return false, datastore.ErrTableNameEmpty
	}

	db := ds.db.WithContext(ctx).Model(entity)
	for key, value := range entity.Index() {
		db = db.Where(fmt.Sprintf("%s = ?", key), value)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check record existence: %v", err)
	}
	return count > 0, nil
}

// Commit commits the transaction
func (ds *SQLite) Commit(ctx context.Context) error {
	return nil
}

// GetDB exposes the underlying *gorm.DB for advanced queries (e.g., vector search)
func (ds *SQLite) GetDB() *gorm.DB {
	return ds.db
}

// getEntityFieldsAndValues gets the fields and values of an entity
func getEntityFieldsAndValues(entity datastore.Entity) ([]string, []interface{}, error) {
	val := reflect.ValueOf(entity).Elem()
	typ := val.Type()

	fields := make([]string, 0, val.NumField())
	values := make([]interface{}, 0, val.NumField())

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)

		// Ignore unexported fields
		if field.PkgPath != "" {
			continue
		}

		fields = append(fields, field.Name)
		values = append(values, value.Interface())
	}

	if len(fields) == 0 {
		return nil, nil, datastore.ErrEntityInvalid
	}
	return fields, values, nil
}

// buildFilterConditions builds filter conditions
func buildFilterConditions(options datastore.FilterOptions) []string {
	filters := make([]string, 0)

	for _, query := range options.Queries {
		filters = append(filters, fmt.Sprintf("%s LIKE '%%%s%%'", query.Key, query.Query))
	}

	for _, in := range options.In {
		quotedValues := make([]string, len(in.Values))
		for i, value := range in.Values {
			quotedValues[i] = "'" + value + "'"
		}
		filters = append(filters, fmt.Sprintf("%s IN (%s)", in.Key, strings.Join(quotedValues, ", ")))
	}

	for _, notExist := range options.IsNotExist {
		filters = append(filters, fmt.Sprintf("%s IS NULL", notExist.Key))
	}

	return filters
}
