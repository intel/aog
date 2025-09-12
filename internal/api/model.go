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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
)

func (t *AOGCoreServer) CreateModel(c *gin.Context) {
	logger.ApiLogger.Debug("[API] CreateModel request params:", c.Request.Body)
	request := new(dto.CreateModelRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		if !errors.Is(err, io.EOF) {
			bcode.ReturnError(c, bcode.ErrModelBadRequest)
			return
		}
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.Model.CreateModel(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] CreateModel response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) DeleteModel(c *gin.Context) {
	logger.ApiLogger.Error("[API] DeleteModel request params:", c.Request.Body)
	request := new(dto.DeleteModelRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.Model.DeleteModel(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] DeleteModel response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) GetModels(c *gin.Context) {
	logger.ApiLogger.Debug("[API] GetModels request params:", c.Request.Body)
	request := new(dto.GetModelsRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := t.Model.GetModels(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] GetModels response:", resp)
	c.JSON(http.StatusOK, resp)
}

func (t *AOGCoreServer) CreateModelStream(c *gin.Context) {
	request := new(dto.CreateModelRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	// c.Writer.Header().Set("Content-Type", "text/event-stream")
	// c.Writer.Header().Set("Cache-Control", "no-cache")
	// c.Writer.Header().Set("Connection", "keep-alive")
	// c.Writer.Header().Set("Transfer-Encoding", "chunked")

	ctx := c.Request.Context()

	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.NotFound(w, c.Request)
		return
	}

	dataCh, errCh := t.Model.CreateModelStream(ctx, request)

	for {
		select {
		case data, ok := <-dataCh:
			if !ok {
				select {
				case err, _ := <-errCh:
					if err != nil {
						fmt.Fprintf(w, "data: {\"status\": \"error\", \"data\":\"%v\"}\n\n", err)
						flusher.Flush()
						return
					}
				}
				if data == nil {
					return
				}
			}

			// 解析Ollama响应
			var resp types.ProgressResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				log.Printf("Error unmarshaling response: %v", err)
				continue
			}

			// 获取响应文本
			// 使用SSE格式发送到前端
			// fmt.Fprintf(w, "data: %s\n\n", response)
			if resp.Completed > 0 || resp.Status == "success" {
				fmt.Fprintf(w, "data: %s\n\n", string(data))
				flusher.Flush()
			}

		case err, _ := <-errCh:
			if err != nil {
				log.Printf("Error: %v", err)
				// 发送错误信息到前端
				if strings.Contains(err.Error(), "context cancel") {
					fmt.Fprintf(w, "data: {\"status\": \"canceled\", \"data\":\"%v\"}\n\n", err)
				} else {
					fmt.Fprintf(w, "data: {\"status\": \"error\", \"data\":\"%v\"}\n\n", err)
				}

				flusher.Flush()
				return
			}

		case <-ctx.Done():
			fmt.Fprintf(w, "data: {\"status\": \"error\", \"data\":\"timeout\"}\n\n")
			flusher.Flush()
			return
		}
	}
}

func (t *AOGCoreServer) CancelModelStream(c *gin.Context) {
	logger.ApiLogger.Error("[API] CancelModelStream request params:", c.Request.Body)
	request := new(dto.ModelStreamCancelRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	ctx := c.Request.Context()
	data, err := t.Model.ModelStreamCancel(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] CancelModelStream response:", data)
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) GetRecommendModels(c *gin.Context) {
	logger.ApiLogger.Debug("[API] GetRecommendModels request params:", c.Request.Body)
	data, err := t.Model.GetRecommendModel()
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] GetRecommendModels response:", data)
	c.JSON(http.StatusOK, data)
}

func (t *AOGCoreServer) GetModelList(c *gin.Context) {
	logger.ApiLogger.Debug("[API] GetModelList request params:", c.Request.Body)
	request := new(dto.GetSupportModelRequest)
	if err := c.Bind(request); err != nil {
		bcode.ReturnError(c, bcode.ErrModelBadRequest)
		return
	}

	if err := validate.Struct(request); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	ctx := c.Request.Context()
	data, err := t.Model.GetSupportModelList(ctx, request)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}

	logger.ApiLogger.Debug("[API] GetModelList response:", data)
	c.JSON(http.StatusOK, data)
}
