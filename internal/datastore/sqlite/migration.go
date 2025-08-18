package sqlite

import (
	"context"
	"fmt"
	"github.com/intel/aog/internal/datastore"
	"reflect"
	"strconv"

	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/types"
	"gorm.io/gorm"
)

var allTables = []interface{}{new(types.ServiceProvider), new(types.Service), new(types.Model), new(types.VersionUpdateRecord), new(types.DataMigrateVersion)}

// VersionManager
type VersionManager interface {
	GetCurrentVersion() (string, error)
	SetCurrentVersion(version string) error
}

type Migration interface {
	Version() string
	GetModifyFields(tableName string) map[string]string
	ExtraDataOperation(ds *SQLite) error
}

// migrationList
var migrationList []Migration

// RegisterMigration
func RegisterMigration(m Migration) {
	migrationList = append(migrationList, m)
}

// MigrateToLatest
func MigrateToLatest(vm VersionManager, ds *SQLite) error {
	initMigrationList()
	if len(migrationList) == 0 {
		err := ds.Init()
		if err != nil {
			return err
		}
		return nil
	}
	err := ds.db.AutoMigrate(&types.DataMigrateVersion{})
	currentVersion, err := vm.GetCurrentVersion()
	if err != nil {
		return err
	}
	start := false
	if currentVersion == "" {
		start = true
	}
	for _, m := range migrationList {
		if !start {
			if m.Version() == currentVersion {
				start = true
			}
			continue
		}
		// migrate
		if err := Migrate(ds, m); err != nil {
			return err
		}
		// custom data migrate
		err = m.ExtraDataOperation(ds)
		if err != nil {
			return err
		}
		// update version
		if err := vm.SetCurrentVersion(m.Version()); err != nil {
			return err
		}
	}
	err = ds.insertInitialData()
	if err != nil {
		return err
	}
	return nil
}

// MigrationV1 example
type MigrationV1 struct{}
type MigrationV2 struct{}
type MigrationV3 struct{}
type MigrationV4 struct{}
type MigrationV5 struct{}

func (m *MigrationV1) GetModifyFields(tableName string) map[string]string {
	return map[string]string{}
}

// The version should match the corresponding AOG version.
func (m *MigrationV1) Version() string { return "v0.1" }

func (m *MigrationV1) ExtraDataOperation(ds *SQLite) error {
	return nil
}

func (m *MigrationV2) GetModifyFields(tableName string) map[string]string {
	return map[string]string{}
}

func (m *MigrationV2) Version() string {
	return "v0.2"
}

func (m *MigrationV2) ExtraDataOperation(ds *SQLite) error {
	return nil
}

func (m *MigrationV3) GetModifyFields(tableName string) map[string]string {
	return map[string]string{}
}

func (m *MigrationV3) Version() string {
	return "v0.3"
}

func (m *MigrationV3) ExtraDataOperation(ds *SQLite) error {
	return nil
}

func (m *MigrationV4) GetModifyFields(tableName string) map[string]string {
	// get modify fields
	switch tableName {
	case types.TableModel:
		return map[string]string{}
	case types.TableServiceProvider:
		return map[string]string{}
	case types.TableService:
		return map[string]string{}
	case types.TableVersionUpdate:
		return map[string]string{}
	default:
		return map[string]string{}
	}
}

func (m *MigrationV4) Version() string {
	return "v0.4"
}

func (m *MigrationV4) ExtraDataOperation(ds *SQLite) error {
	// model add new field
	ctx := context.Background()
	modelQ := new(types.Model)
	models, err := ds.List(ctx, modelQ, &datastore.ListOptions{})
	if err != nil {
		return err
	}
	// update model data
	for _, modelObj := range models {
		model := modelObj.(*types.Model)
		sp := new(types.ServiceProvider)
		sp.ProviderName = model.ProviderName
		err = ds.Get(ctx, sp)
		if err != nil {
			return err
		}
		model.ServiceName = sp.ServiceName
		model.ServiceSource = sp.ServiceSource
		err = ds.Put(ctx, model)
		if err != nil {
			return err
		}
	}
	// update service data
	serviceQ := new(types.Service)
	services, err := ds.List(ctx, serviceQ, &datastore.ListOptions{})
	if err != nil {
		return err
	}
	for _, serviceObj := range services {
		updateStatus := 0
		service := serviceObj.(*types.Service)
		if service.Avatar == "" {
			updateStatus = 1
			service.Avatar = types.SupportServiceAvatar[service.Name]
		}
		if service.Status == 0 {
			service.Status = -1
			service.CanInstall = 1
			updateStatus = 1
		}
		if updateStatus == 1 {
			err = ds.Put(ctx, service)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MigrationV5) GetModifyFields(tableName string) map[string]string {
	return map[string]string{}
}

func (m *MigrationV5) Version() string {
	return "v0.5"
}
func (m *MigrationV5) ExtraDataOperation(ds *SQLite) error {
	return nil
}

func Migrate(ds *SQLite, m Migration) error {
	fmt.Println("db is nil:", ds.db == nil)
	return ds.db.Transaction(func(tx *gorm.DB) error {
		for _, table := range allTables {
			var tableName string
			switch tableType := table.(type) {
			case *types.ServiceProvider:
				// get old data
				tableName = tableType.TableName()
				modifyFields := m.GetModifyFields(tableName)
				if len(modifyFields) == 0 {
					err := tx.Migrator().AutoMigrate(&types.ServiceProvider{})
					if err != nil {
						return err
					}
					continue
				}
				var oldDataRows []map[string]interface{}
				tx.Table(tableName).Find(&oldDataRows)
				spDataList := make([]*types.ServiceProvider, 0)
				// migrate old data -> new tmp data
				for _, oldDataRow := range oldDataRows {
					sp := &types.ServiceProvider{}
					setField(sp, oldDataRow, modifyFields)
					spDataList = append(spDataList, sp)
				}
				// rename old table name
				err := tx.Migrator().RenameTable(tableName, tableName+"_old")
				if err != nil {
					logger.LogicLogger.Error("[Migrate running] rename table err", "err", err)
					return err
				}
				if err = tx.Migrator().AutoMigrate(&types.ServiceProvider{}); err != nil {
					logger.LogicLogger.Error("[Migrate running] auto migrate err", "err", err)
					return err
				}
				if err = tx.CreateInBatches(spDataList, len(spDataList)).Error; err != nil {
					logger.LogicLogger.Error("[Migrate running] failed to migrate data service_provider : %v", err)
					return err
				}
				// drop old table
				if err = tx.Migrator().DropTable(tableName + "_old"); err != nil {
					logger.LogicLogger.Error("[Migrate running] drop table err", "err", err)
					return err
				}
			case *types.Model:
				tableName = tableType.TableName()
				modifyFields := m.GetModifyFields(tableName)
				if len(modifyFields) == 0 {
					err := tx.Migrator().AutoMigrate(&types.Model{})
					if err != nil {
						return err
					}
					continue
				}
				var oldDataRows []map[string]interface{}
				tx.Table(tableName).Find(&oldDataRows)
				mDataList := make([]*types.ServiceProvider, 0)

				for _, oldDataRow := range oldDataRows {
					sp := &types.ServiceProvider{}
					setField(sp, oldDataRow, modifyFields)
					mDataList = append(mDataList, sp)
				}
				// rename old table name
				err := tx.Migrator().RenameTable(tableName, tableName+"_old")
				if err != nil {
					logger.LogicLogger.Error("[Migrate running] rename table err", "err", err)
					return err
				}
				if err = tx.Migrator().AutoMigrate(&types.Model{}); err != nil {
					logger.LogicLogger.Error("[Migrate running] auto migrate err", "err", err)
					return err
				}
				if err = tx.CreateInBatches(mDataList, len(mDataList)).Error; err != nil {
					logger.LogicLogger.Error("[Migrate running] failed to migrate data model : %v", err)
					return err
				}
				// drop old table
				if err = tx.Migrator().DropTable(tableName + "_old"); err != nil {
					logger.LogicLogger.Error("[Migrate running] drop table err", "err", err)
					return err
				}
			case *types.VersionUpdateRecord:
				tableName = tableType.TableName()
				modifyFields := m.GetModifyFields(tableName)
				if len(modifyFields) == 0 {
					err := tx.Migrator().AutoMigrate(&types.VersionUpdateRecord{})
					if err != nil {
						return err
					}
					continue
				}
				var oldDataRows []map[string]interface{}
				tx.Table(tableName).Find(&oldDataRows)
				vDataList := make([]*types.ServiceProvider, 0)
				for _, oldDataRow := range oldDataRows {
					sp := &types.ServiceProvider{}
					setField(sp, oldDataRow, modifyFields)
					vDataList = append(vDataList, sp)
				}
				// rename old table name
				err := tx.Migrator().RenameTable(tableName, tableName+"_old")
				if err != nil {
					logger.LogicLogger.Error("[Migrate running] rename table err", "err", err)
					return err
				}
				if err = tx.Migrator().AutoMigrate(&types.Model{}); err != nil {
					logger.LogicLogger.Error("[Migrate running] auto migrate err", "err", err)
					return err
				}
				if err = tx.CreateInBatches(vDataList, len(vDataList)).Error; err != nil {
					logger.LogicLogger.Error("[Migrate running] failed to migrate data version record : %v", err)
					return err
				}
				// drop old table
				if err = tx.Migrator().DropTable(tableName + "_old"); err != nil {
					logger.LogicLogger.Error("[Migrate running] drop table err", "err", err)
					return err
				}
			case *types.Service:
				tableName = tableType.TableName()
				modifyFields := m.GetModifyFields(tableName)
				if len(modifyFields) == 0 {
					err := tx.Migrator().AutoMigrate(&types.Service{})
					if err != nil {
						return err
					}
					continue
				}
				var oldDataRows []map[string]interface{}
				tx.Table(tableName).Find(&oldDataRows)
				serviceDataList := make([]*types.ServiceProvider, 0)
				for _, oldDataRow := range oldDataRows {
					sp := &types.ServiceProvider{}
					setField(sp, oldDataRow, modifyFields)
					serviceDataList = append(serviceDataList, sp)
				}
				// rename old table name
				err := tx.Migrator().RenameTable(tableName, tableName+"_old")
				if err != nil {
					logger.LogicLogger.Error("[Migrate running] rename table err", "err", err)
					return err
				}
				if err = tx.Migrator().AutoMigrate(&types.Service{}); err != nil {
					logger.LogicLogger.Error("[Migrate running] auto migrate err", "err", err)
					return err
				}
				if err = tx.CreateInBatches(serviceDataList, len(serviceDataList)).Error; err != nil {
					logger.LogicLogger.Error("[Migrate running] failed to migrate data service : %v", err)
					return err
				}
				// drop old table
				if err = tx.Migrator().DropTable(tableName + "_old"); err != nil {
					logger.LogicLogger.Error("[Migrate running] drop table err", "err", err)
					return err
				}
			}
		}
		return nil
	})
}

func setField(ptr interface{}, values map[string]interface{}, fieldMap map[string]string) {
	v := reflect.ValueOf(ptr).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		newField := t.Field(i).Name
		oldField, ok := fieldMap[newField]
		if !ok {
			oldField = newField // 没有映射则用同名
		}
		if val, exists := values[oldField]; exists {
			field := v.FieldByName(newField)
			if field.IsValid() && field.CanSet() {
				converted, err := convertType(val, field.Type())
				if err == nil {
					field.Set(converted)
				} else {
					// 可以加日志
					fmt.Printf("字段 %s 类型转换失败: %v\n", newField, err)
				}
			}
		}
	}
}

func convertType(val interface{}, targetType reflect.Type) (reflect.Value, error) {
	v := reflect.ValueOf(val)
	if v.Type().AssignableTo(targetType) {
		return v.Convert(targetType), nil
	}
	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(fmt.Sprintf("%v", val)), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v.Kind() {
		case reflect.Float64:
			return reflect.ValueOf(int(v.Float())), nil
		case reflect.String:
			i, err := strconv.ParseInt(v.String(), 10, 64)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(i).Convert(targetType), nil
		}
	case reflect.Float32, reflect.Float64:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(float64(v.Int())).Convert(targetType), nil
		case reflect.String:
			f, err := strconv.ParseFloat(v.String(), 64)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(f).Convert(targetType), nil
		}
	case reflect.Bool:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return reflect.ValueOf(v.Int() != 0), nil
		case reflect.String:
			b, err := strconv.ParseBool(v.String())
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(b), nil
		}
	}
	return reflect.Value{}, fmt.Errorf("不支持的类型转换: %v -> %v", v.Type(), targetType)
}

// 在 init 函数中注册迁移
func initMigrationList() {
	RegisterMigration(&MigrationV1{})
	RegisterMigration(&MigrationV2{})
	RegisterMigration(&MigrationV3{})
	RegisterMigration(&MigrationV4{})
	RegisterMigration(&MigrationV5{})
}

// SQLiteVersionManager 实现 VersionManager 接口，基于 version_update_record 表
type SQLiteVersionManager struct {
	db *gorm.DB
}

func NewSQLiteVersionManager(ds *SQLite) *SQLiteVersionManager {
	return &SQLiteVersionManager{db: ds.db}
}

func (vm *SQLiteVersionManager) GetCurrentVersion() (string, error) {
	var record struct {
		Version string `gorm:"column:version"`
	}
	var currentVersion string
	err := vm.db.Table(types.TableDataMigrateVersion).Order("id desc").Limit(1).Find(&record).Error
	if err != nil {
		currentVersion = "v0.1"
	} else {
		currentVersion = record.Version
	}
	return currentVersion, nil
}

func (vm *SQLiteVersionManager) SetCurrentVersion(version string) error {
	record := map[string]interface{}{"version": version, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}
	return vm.db.Table(types.TableDataMigrateVersion).Create(record).Error
}
