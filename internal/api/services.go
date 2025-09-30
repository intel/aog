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
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/utils/bcode"
)

func (t *AOGCoreServer) CreateAIGCService(c *gin.Context) {
	logger.ApiLogger.Debug("[API] CreateAIGCService request params:", c.Request.Body)
	request := new(dto.CreateAIGCServiceRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrAIGCServiceBadRequest)
		return
	}

	if err := ValidateAndSetDefaults(request); err != nil {
		logger.ApiLogger.Warn("[API] CreateAIGCService validation failed:", err)
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.AIGCService.CreateAIGCService(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] CreateAIGCService response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) UpdateAIGCService(c *gin.Context) {
	logger.ApiLogger.Debug("[API] UpdateAIGCService request params:", c.Request.Body)
	request := new(dto.UpdateAIGCServiceRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrAIGCServiceBadRequest)
		return
	}

	if err := ValidateAndSetDefaults(request); err != nil {
		logger.ApiLogger.Warn("[API] UpdateAIGCService validation failed:", err)
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.AIGCService.UpdateAIGCService(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] UpdateAIGCService response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) GetAIGCService(c *gin.Context) {
}

func (t *AOGCoreServer) GetAIGCServices(c *gin.Context) {
	logger.ApiLogger.Debug("[API] GetAIGCServices request params:", c.Request.Body)
	request := new(dto.GetAIGCServicesRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		if !errors.Is(err, io.EOF) {
			bcode.ReturnError(c, bcode.ErrAIGCServiceBadRequest)
			return
		}
	}

	if err := ValidateAndSetDefaults(request); err != nil {
		logger.ApiLogger.Warn("[API] GetAIGCServices validation failed:", err)
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.AIGCService.GetAIGCServices(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] GetAIGCServices response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) ExportService(c *gin.Context) {
	logger.ApiLogger.Debug("[API] ExportService request params:", c.Request.Body)
	request := new(dto.ExportServiceRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrAIGCServiceBadRequest)
		return
	}

	if err := ValidateAndSetDefaults(request); err != nil {
		logger.ApiLogger.Warn("[API] ExportService validation failed:", err)
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.AIGCService.ExportService(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] ExportService response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) ImportService(c *gin.Context) {
	logger.ApiLogger.Debug("[API] ImportService request params:", c.Request.Body)
	request := new(dto.ImportServiceRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrAIGCServiceBadRequest)
		return
	}

	if err := ValidateAndSetDefaults(request); err != nil {
		logger.ApiLogger.Warn("[API] ImportService validation failed:", err)
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.AIGCService.ImportService(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] ImportService response:", resp)
	c.JSON(http.StatusOK, resp)
}
