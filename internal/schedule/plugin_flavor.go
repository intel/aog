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

package schedule

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/intel/aog/internal/convert"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/types"
	sdktypes "github.com/intel/aog/plugin-sdk/types"
)

// PluginBasedAPIFlavor 基于插件 manifest 动态创建的 APIFlavor
// 实现与内置 ConfigBasedAPIFlavor 相同的接口
type PluginBasedAPIFlavor struct {
	manifest  *sdktypes.PluginManifest
	pipelines map[string]map[string]*convert.ConverterPipeline
}

// InitServiceDefaultInfo 初始化插件的服务默认信息
// 插件需要手动初始化服务信息，因为不依赖内置 template
func (f *PluginBasedAPIFlavor) InitServiceDefaultInfo() {
	serviceInfoMap := make(map[string]ServiceDefaultInfo)

	for _, svc := range f.manifest.Services {
		// 构建完整的 URL（对于 local 类型插件）
		fullURL := svc.Endpoint
		if f.manifest.Provider.Type == "local" {
			if f.manifest.Provider.EngineHost != "" {
				fullURL = f.manifest.Provider.EngineHost + svc.Endpoint
			}
		}

		// 从 Protocol 推断 HTTP 方法
		method := f.getHTTPMethodFromProtocol(svc.Protocol)

		// 构建 Endpoints 列表（格式: "METHOD PATH"）
		endpoints := []string{
			fmt.Sprintf("%s %s", method, svc.Endpoint),
		}

		serviceInfoMap[svc.ServiceName] = ServiceDefaultInfo{
			Endpoints:       endpoints,
			DefaultModel:    svc.DefaultModel,
			RequestUrl:      fullURL,
			RequestExtraUrl: "", // 插件不需要 extra URL
			AuthType:        svc.AuthType,
			RequestSegments: 0,  // 插件不使用分段请求
			ExtraHeaders:    "", // 插件不需要额外 headers
			SupportModels:   svc.SupportModels,
			AuthApplyUrl:    "", // 插件不需要 auth apply URL
			Protocol:        svc.Protocol,
			TaskType:        svc.TaskType,
			ExposeProtocol:  svc.ExposeProtocol,
		}
	}

	FlavorServiceDefaultInfoMap[f.manifest.Provider.Name] = serviceInfoMap

	logger.EngineLogger.Debug("Initialized plugin service default info",
		"plugin", f.manifest.Provider.Name,
		"services", len(serviceInfoMap))
}

// getHTTPMethodFromProtocol 从协议类型推断 HTTP 方法
func (f *PluginBasedAPIFlavor) getHTTPMethodFromProtocol(protocol string) string {
	protocol = strings.ToUpper(protocol)
	switch protocol {
	case "HTTP", "HTTPS":
		return "POST" // 默认使用 POST
	case "GRPC", "GRPC_STREAM":
		return "POST" // gRPC 也映射为 POST
	case "WEBSOCKET":
		return "GET" // WebSocket 通常用 GET 升级
	default:
		return "POST" // 默认返回 POST
	}
}

// NewPluginBasedAPIFlavor 从插件 manifest 创建 APIFlavor
func NewPluginBasedAPIFlavor(manifest *sdktypes.PluginManifest) (*PluginBasedAPIFlavor, error) {
	logger.EngineLogger.Info("Creating plugin-based API flavor",
		"plugin", manifest.Provider.Name,
		"services", len(manifest.Services))

	flavor := &PluginBasedAPIFlavor{
		manifest:  manifest,
		pipelines: make(map[string]map[string]*convert.ConverterPipeline),
	}

	for _, svc := range manifest.Services {
		serviceName := svc.ServiceName

		flavor.pipelines[serviceName] = make(map[string]*convert.ConverterPipeline)

		if svc.ConfigRef == "" {
			return nil, fmt.Errorf("service %s missing required config_ref", serviceName)
		}

		logger.EngineLogger.Debug("Loading conversion from template reference",
			"plugin", manifest.Provider.Name,
			"service", serviceName,
			"config_ref", svc.ConfigRef)

		if err := flavor.loadPipelinesFromTemplate(&svc); err != nil {
			return nil, fmt.Errorf("failed to load conversion from template %s for service %s: %w",
				svc.ConfigRef, serviceName, err)
		}
	}

	logger.EngineLogger.Info("Plugin-based API flavor created successfully",
		"plugin", manifest.Provider.Name,
		"services", len(flavor.pipelines))

	flavor.InitServiceDefaultInfo()

	return flavor, nil
}

// loadPipelinesFromTemplate 从 template 加载转换管道
// 支持两种来源：1. 内置 template  2. 插件本地 template
func (f *PluginBasedAPIFlavor) loadPipelinesFromTemplate(svc *sdktypes.ServiceDef) error {
	// 解析 ConfigRef: "flavor:service"
	parts := strings.Split(svc.ConfigRef, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid config_ref format: %s, expected 'flavor:service'", svc.ConfigRef)
	}

	flavorName := parts[0]
	templateServiceName := parts[1]

	// 1. 优先尝试从内置 template 加载
	flavorDef := GetFlavorDef(flavorName)
	if flavorDef.Name == "" {
		// 2. 尝试从插件本地 template 加载
		logger.EngineLogger.Debug("Built-in flavor not found, trying plugin-local template",
			"flavor", flavorName,
			"plugin", f.manifest.Provider.Name)

		pluginFlavorDef, err := f.loadPluginLocalTemplate(flavorName)
		if err != nil {
			return fmt.Errorf("template not found: %s (neither built-in nor plugin-local): %w", flavorName, err)
		}
		flavorDef = *pluginFlavorDef // ✅ 解引用指针
	}

	// 获取引用的服务定义
	templateService, exists := flavorDef.Services[templateServiceName]
	if !exists {
		return fmt.Errorf("service %s not found in flavor %s", templateServiceName, flavorName)
	}

	// 复制所有转换管道
	conversions := map[string]types.FlavorConversionDef{
		"request_to_aog":           templateService.RequestToAOG,
		"request_from_aog":         templateService.RequestFromAOG,
		"response_to_aog":          templateService.ResponseToAOG,
		"response_from_aog":        templateService.ResponseFromAOG,
		"stream_response_to_aog":   templateService.StreamResponseToAOG,
		"stream_response_from_aog": templateService.StreamResponseFromAOG,
	}

	for convName, convDef := range conversions {
		if len(convDef.Conversion) == 0 {
			continue
		}

		pipeline, err := convert.NewConverterPipeline(convDef.Conversion)
		if err != nil {
			return fmt.Errorf("failed to create pipeline for %s: %w", convName, err)
		}

		f.pipelines[svc.ServiceName][convName] = pipeline

		logger.EngineLogger.Debug("Loaded conversion pipeline from template",
			"service", svc.ServiceName,
			"conversion", convName,
			"steps", len(convDef.Conversion))
	}

	return nil
}

// loadPluginLocalTemplate 从插件目录加载本地 template
// 例如：~/.aog/plugins/doubao-plugin/templates/doubao.yaml
func (f *PluginBasedAPIFlavor) loadPluginLocalTemplate(flavorName string) (*FlavorDef, error) {
	if f.manifest.PluginDir == "" {
		return nil, fmt.Errorf("plugin directory not set in manifest")
	}

	// 查找 template 文件
	templateDir := filepath.Join(f.manifest.PluginDir, "templates")
	templateFile := filepath.Join(templateDir, flavorName+".yaml")

	// 检查文件是否存在
	if _, err := os.Stat(templateFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("template file not found: %s", templateFile)
	}

	// 读取文件
	data, err := os.ReadFile(templateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	// 解析 YAML
	var flavorDef FlavorDef
	if err := yaml.Unmarshal(data, &flavorDef); err != nil {
		return nil, fmt.Errorf("failed to parse template file: %w", err)
	}

	logger.EngineLogger.Info("Loaded plugin-local template",
		"plugin", f.manifest.Provider.Name,
		"flavor", flavorName,
		"file", templateFile,
		"services", len(flavorDef.Services))

	return &flavorDef, nil
}

// getTemplateService 获取 template 中的服务定义（公共方法，避免重复代码）
func (f *PluginBasedAPIFlavor) getTemplateService(serviceName string) (*FlavorServiceDef, error) {
	svcDef, err := f.manifest.GetServiceByName(serviceName)
	if err != nil {
		return nil, err
	}

	if svcDef.ConfigRef == "" {
		return nil, fmt.Errorf("no config_ref for service: %s", serviceName)
	}

	// 解析 ConfigRef
	parts := strings.Split(svcDef.ConfigRef, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid config_ref: %s", svcDef.ConfigRef)
	}

	flavorName := parts[0]
	templateServiceName := parts[1]

	// 获取 flavor 定义（优先内置，然后插件本地）
	flavorDef := GetFlavorDef(flavorName)
	if flavorDef.Name == "" {
		// 尝试加载插件本地 template
		pluginFlavorDef, err := f.loadPluginLocalTemplate(flavorName)
		if err != nil {
			return nil, fmt.Errorf("template not found: %s", flavorName)
		}
		flavorDef = *pluginFlavorDef // ✅ 解引用指针
	}

	// 获取服务定义
	templateSvc, exists := flavorDef.Services[templateServiceName]
	if !exists {
		return nil, fmt.Errorf("service %s not found in template %s", templateServiceName, flavorName)
	}

	return &templateSvc, nil
}

// ========== 实现 APIFlavor 接口 ==========

func (f *PluginBasedAPIFlavor) Name() string {
	return f.manifest.Provider.Name
}

func (f *PluginBasedAPIFlavor) InstallRoutes(server *gin.Engine) {
	// Plugin 不需要安装路由，由 AOG 统一处理
	logger.EngineLogger.Debug("Plugin does not install routes",
		"plugin", f.manifest.Provider.Name)
}

func (f *PluginBasedAPIFlavor) GetStreamResponseProlog(service string) []string {
	templateSvc, err := f.getTemplateService(service)
	if err != nil {
		logger.EngineLogger.Debug("Failed to get template service for prologue",
			"service", service,
			"error", err)
		return nil
	}
	return templateSvc.StreamResponseToAOG.Prologue
}

func (f *PluginBasedAPIFlavor) GetStreamResponseEpilog(service string) []string {
	templateSvc, err := f.getTemplateService(service)
	if err != nil {
		logger.EngineLogger.Debug("Failed to get template service for epilogue",
			"service", service,
			"error", err)
		return nil
	}
	return templateSvc.StreamResponseToAOG.Epilogue
}

func (f *PluginBasedAPIFlavor) Convert(service string, conversion string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	// 获取对应的转换管道
	servicePipelines, exists := f.pipelines[service]
	if !exists {
		return content, fmt.Errorf("service %s not found in plugin %s", service, f.manifest.Provider.Name)
	}

	pipeline, exists := servicePipelines[conversion]
	if !exists {
		// 如果没有定义转换管道，返回原始内容（无需转换）
		logger.EngineLogger.Debug("No conversion pipeline defined, returning original content",
			"plugin", f.manifest.Provider.Name,
			"service", service,
			"conversion", conversion)
		return content, nil
	}

	// 应用转换管道
	convertedContent, err := pipeline.Convert(content, ctx)
	if err != nil {
		return types.HTTPContent{}, fmt.Errorf("conversion failed for %s.%s: %w",
			service, conversion, err)
	}

	logger.EngineLogger.Debug("Conversion applied successfully",
		"plugin", f.manifest.Provider.Name,
		"service", service,
		"conversion", conversion)

	return convertedContent, nil
}

func (f *PluginBasedAPIFlavor) ConvertRequestToAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "request_to_aog", content, ctx)
}

func (f *PluginBasedAPIFlavor) ConvertRequestFromAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "request_from_aog", content, ctx)
}

func (f *PluginBasedAPIFlavor) ConvertResponseToAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "response_to_aog", content, ctx)
}

func (f *PluginBasedAPIFlavor) ConvertResponseFromAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "response_from_aog", content, ctx)
}

func (f *PluginBasedAPIFlavor) ConvertStreamResponseToAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "stream_response_to_aog", content, ctx)
}

func (f *PluginBasedAPIFlavor) ConvertStreamResponseFromAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "stream_response_from_aog", content, ctx)
}
