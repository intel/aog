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

package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/version"
)

func InjectRouter(e *AOGCoreServer) {
	e.Router.Handle(http.MethodGet, "/", rootHandler)
	e.Router.Handle(http.MethodGet, "/health", e.GetServerHealth)
	e.Router.Handle(http.MethodGet, "/engine/health", e.GetEngineServerHealth)
	e.Router.Handle(http.MethodGet, "/version", e.GetVersion)
	e.Router.Handle(http.MethodGet, "/engine/version", e.GetEngineVersion)
	e.Router.Handle(http.MethodGet, "/update/status", e.GetUpdateAvailableStatus)
	e.Router.Handle(http.MethodPost, "/update", e.UpdateHandler)

	r := e.Router.Group("/" + constants.AppName + "/" + version.SpecVersion)

	// service import / export
	r.Handle(http.MethodPost, "/service/export", e.ExportService)
	r.Handle(http.MethodPost, "/service/import", e.ImportService)

	// Inject the router into the server
	r.Handle(http.MethodPost, "/service/install", e.CreateAIGCService)
	r.Handle(http.MethodPut, "/service", e.UpdateAIGCService)
	r.Handle(http.MethodGet, "/service", e.GetAIGCServices)

	r.Handle(http.MethodGet, "/service_provider", e.GetServiceProviders)
	r.Handle(http.MethodGet, "/service_provider/detail", e.GetServiceProvider)
	r.Handle(http.MethodPost, "/service_provider", e.CreateServiceProvider)
	r.Handle(http.MethodPut, "/service_provider", e.UpdateServiceProvider)
	r.Handle(http.MethodDelete, "/service_provider", e.DeleteServiceProvider)

	r.Handle(http.MethodGet, "/model", e.GetModels)
	r.Handle(http.MethodPost, "/model", e.CreateModel)
	r.Handle(http.MethodDelete, "/model", e.DeleteModel)
	r.Handle(http.MethodPost, "/model/stream", e.CreateModelStream)
	r.Handle(http.MethodPost, "/model/stream/cancel", e.CancelModelStream)
	r.Handle(http.MethodGet, "/model/recommend", e.GetRecommendModels)
	r.Handle(http.MethodGet, "/model/support", e.GetModelList)

	r.Handle(http.MethodGet, "/control_panel/dashboard", e.GetDashBoardHandler)
	r.Handle(http.MethodGet, "/control_panel/modellist", e.GetSupportModelListCombine)
	r.Handle(http.MethodPost, "/control_panel/set_default", e.SetDefaultModelHandler)
	r.Handle(http.MethodGet, "/control_panel/about", e.GetProductInfoHandler)
	r.Handle(http.MethodPost, "/control_panel/modelkey", e.GetModelkeyHandler)

	// rag service
	r.Handle(http.MethodGet, "/rag/file/detail", e.RagGetFile)
	r.Handle(http.MethodGet, "/rag/file", e.RagGetFiles)
	r.Handle(http.MethodPost, "/rag/file", e.RagUploadFile)
	r.Handle(http.MethodDelete, "/rag/file", e.RagDeleteFile)
	r.Handle(http.MethodPost, "/rag/retrieval", e.RagRetrieval)

	// plugin service
	p := r.Group("/plugin")
	p.Handle(http.MethodGet, "/list", e.PluginList)
	p.Handle(http.MethodGet, "/info", e.PluginInfo)
	p.Handle(http.MethodDelete, "/delete", e.PluginDelete)
	p.Handle(http.MethodPost, "/stop", e.PluginStop)
	p.Handle(http.MethodPost, "/load", e.PluginLoad)

	slog.Info("Gateway started", "host", config.GlobalEnvironment.ApiHost)
}

func rootHandler(c *gin.Context) {
	c.String(http.StatusOK, "AIPC Open Gateway")
}
