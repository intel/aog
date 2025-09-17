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

package types

import (
	"time"

	"github.com/intel/aog/internal/datastore"
)

const (
	// Database table names
	TableService            = "aog_service"
	TableServiceProvider    = "aog_service_provider"
	TableModel              = "aog_model"
	TableVersionUpdate      = "aog_version_update_record"
	TableDataMigrateVersion = "aog_data_migration_version"
	// RAG tables
	TableRagFile   = "aog_rag_file"
	TableRagChunk  = "aog_rag_chunk"
	TableRagVector = "aog_rag_vector"
)

// Service  table structure
type Service struct {
	Name           string    `gorm:"primaryKey;column:name" json:"name"`
	HybridPolicy   string    `gorm:"column:hybrid_policy;not null;default:default" json:"hybrid_policy"`
	RemoteProvider string    `gorm:"column:remote_provider;not null;default:''" json:"remote_provider"` // v0.6 deprecated
	LocalProvider  string    `gorm:"column:local_provider;not null;default:''" json:"local_provider"`   // v0.6 deprecated
	Status         int       `gorm:"column:status;not null;default:1" json:"status"`
	CanInstall     int       `gorm:"column:can_install;not null;default:0" json:"can_install"`
	Avatar         string    `gorm:"column:avatar;not null;default:''" json:"avatar"`
	CreatedAt      time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (t *Service) SetCreateTime(time time.Time) {
	t.CreatedAt = time
}

func (t *Service) SetUpdateTime(time time.Time) {
	t.UpdatedAt = time
}

func (t *Service) PrimaryKey() string {
	return "name"
}

func (t *Service) TableName() string {
	return TableService
}

func (t *Service) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if t.Name != "" {
		index["name"] = t.Name
	}

	return index
}

// ServiceProvider Service provider table structure
type ServiceProvider struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	ProviderName  string    `gorm:"column:provider_name" json:"provider_name"`
	ServiceName   string    `gorm:"column:service_name" json:"service_name"`
	ServiceSource string    `gorm:"column:service_source;default:local" json:"service_source"`
	Desc          string    `gorm:"column:desc" json:"desc"`
	Method        string    `gorm:"column:method" json:"method"`
	URL           string    `gorm:"column:url" json:"url"`
	AuthType      string    `gorm:"column:auth_type" json:"auth_type"`
	AuthKey       string    `gorm:"column:auth_key" json:"auth_key"`
	Flavor        string    `gorm:"column:flavor" json:"flavor"`
	ExtraHeaders  string    `gorm:"column:extra_headers;default:'{}'" json:"extra_headers"`
	ExtraJSONBody string    `gorm:"column:extra_json_body;default:'{}'" json:"extra_json_body"`
	Properties    string    `gorm:"column:properties;default:'{}'" json:"properties"`
	Status        int       `gorm:"column:status;not null;default:0" json:"status"`
	Scope         string    `gorm:"column:scope;default:system" json:"scope"` // 'system' or 'custom'
	CreatedAt     time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (t *ServiceProvider) SetCreateTime(time time.Time) {
	t.CreatedAt = time
}

func (t *ServiceProvider) SetUpdateTime(time time.Time) {
	t.UpdatedAt = time
}

func (t *ServiceProvider) PrimaryKey() string {
	return "id"
}

func (t *ServiceProvider) TableName() string {
	return TableServiceProvider
}

func (t *ServiceProvider) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if t.ProviderName != "" {
		index["provider_name"] = t.ProviderName
	}

	if t.ServiceSource != "" {
		index["service_source"] = t.ServiceSource
	}

	if t.ServiceName != "" {
		index["service_name"] = t.ServiceName
	}

	if t.Flavor != "" {
		index["flavor"] = t.Flavor
	}
	return index
}

// Model  table structure
type Model struct {
	ID            int       `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	ModelName     string    `gorm:"column:model_name;not null" json:"model_name"`
	ProviderName  string    `gorm:"column:provider_name" json:"provider_name"`
	Status        string    `gorm:"column:status;not null" json:"status"`
	ServiceName   string    `gorm:"column:service_name" json:"service_name"`
	ServiceSource string    `gorm:"column:service_source" json:"service_source"`
	IsDefault     bool      `gorm:"column:is_default" json:"is_default"`
	CreatedAt     time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (t *Model) SetCreateTime(time time.Time) {
	t.CreatedAt = time
}

func (t *Model) SetUpdateTime(time time.Time) {
	t.UpdatedAt = time
}

func (t *Model) PrimaryKey() string {
	return "id"
}

func (t *Model) TableName() string {
	return TableModel
}

func (t *Model) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if t.ModelName != "" {
		index["model_name"] = t.ModelName
	}

	if t.ProviderName != "" {
		index["provider_name"] = t.ProviderName
	}

	if t.ServiceName != "" {
		index["service_name"] = t.ServiceName
	}

	return index
}

// VersionUpdateRecord  table structure
type VersionUpdateRecord struct {
	ID           int       `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	Version      string    `gorm:"column:version;not null" json:"version"`
	ReleaseNotes string    `gorm:"column:release_notes;not null" json:"release_notes"`
	Status       int       `gorm:"column:status;not null" json:"status"`
	CreatedAt    time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (t *VersionUpdateRecord) SetCreateTime(time time.Time) {
	t.CreatedAt = time
}

func (t *VersionUpdateRecord) SetUpdateTime(time time.Time) {
	t.UpdatedAt = time
}

func (t *VersionUpdateRecord) PrimaryKey() string {
	return "id"
}

func (t *VersionUpdateRecord) TableName() string {
	return TableVersionUpdate
}

func (t *VersionUpdateRecord) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if t.Version != "" {
		index["version"] = t.Version
	}

	return index
}

// VersionUpdateRecord  table structure
type DataMigrateVersion struct {
	ID        int       `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	Version   string    `gorm:"column:version;not null" json:"version"`
	CreatedAt time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (t *DataMigrateVersion) SetCreateTime(time time.Time) {
	t.CreatedAt = time
}

func (t *DataMigrateVersion) SetUpdateTime(time time.Time) {
	t.UpdatedAt = time
}

func (t *DataMigrateVersion) PrimaryKey() string {
	return "id"
}

func (t *DataMigrateVersion) TableName() string {
	return TableDataMigrateVersion
}

func (t *DataMigrateVersion) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if t.Version != "" {
		index["version"] = t.Version
	}

	return index
}

// RagFile represents an uploaded document for RAG
type RagFile struct {
	ID         int       `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	FileName   string    `gorm:"column:file_name" json:"file_name"`
	FileID     string    `gorm:"column:file_id" json:"file_id"`
	FileType   string    `gorm:"column:file_type" json:"file_type"`
	FilePath   string    `gorm:"column:file_path" json:"file_path"`
	Status     int       `gorm:"column:status" json:"status"` // 1-processing | 2-done | 3-failed
	EmbedModel string    `gorm:"column:embed_model" json:"embed_model"`
	CreatedAt  time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (t *RagFile) SetCreateTime(tm time.Time) { t.CreatedAt = tm }
func (t *RagFile) SetUpdateTime(tm time.Time) { t.UpdatedAt = tm }
func (t *RagFile) PrimaryKey() string         { return "id" }
func (t *RagFile) TableName() string          { return TableRagFile }
func (t *RagFile) Index() map[string]interface{} {
	idx := make(map[string]interface{})
	if t.FileName != "" {
		idx["file_name"] = t.FileName
	}
	if t.FileID != "" {
		idx["file_id"] = t.FileID
	}
	return idx
}

// RagChunk represents a text chunk of a RagFile
type RagChunk struct {
	ID         string                `json:"id"`
	FileID     string                `json:"file_id"`
	Content    string                `json:"content"`
	ChunkIndex int                   `json:"index"`
	Embedding  datastore.Float32List `json:"embedding"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

func (t *RagChunk) SetCreateTime(tm time.Time) { t.CreatedAt = tm }
func (t *RagChunk) SetUpdateTime(tm time.Time) { t.UpdatedAt = tm }
func (t *RagChunk) PrimaryKey() string         { return "id" }
func (t *RagChunk) TableName() string          { return TableRagChunk }
func (t *RagChunk) Index() map[string]interface{} {
	idx := make(map[string]interface{})
	if t.ID != "" {
		idx["id"] = t.ID
	}
	if t.FileID != "" {
		idx["file_id"] = t.FileID
	}
	return idx
}
