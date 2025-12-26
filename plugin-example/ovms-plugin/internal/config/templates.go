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

package config

import (
	"strings"
)

// Graph.pbtxt templates for different model types
// These templates define the mediapipe computation graphs for OVMS

const (
	// GraphPBTxtChat defines the graph for chat/completion tasks
	GraphPBTxtChat = `input_stream: "HTTP_REQUEST_PAYLOAD:input"
output_stream: "HTTP_RESPONSE_PAYLOAD:output"

node: {
  name: "LLMExecutor"
  calculator: "HttpLLMCalculator"
  input_stream: "LOOPBACK:loopback"
  input_stream: "HTTP_REQUEST_PAYLOAD:input"
  input_side_packet: "LLM_NODE_RESOURCES:llm"
  output_stream: "LOOPBACK:loopback"
  output_stream: "HTTP_RESPONSE_PAYLOAD:output"
  input_stream_info: {
    tag_index: 'LOOPBACK:0',
    back_edge: true
  }
  node_options: {
      [type.googleapis.com / mediapipe.LLMCalculatorOptions]: {
          models_path: "%s/models/%s",
          plugin_config: '{}',
          enable_prefix_caching: false,
          cache_size: 1,
          max_num_batched_tokens: 8192,
          dynamic_split_fuse: false, 
          max_num_seqs: 256,
          device: "GPU",
          tool_parser: "%s",
          reasoning_parser: "%s",
      }
  }
  input_stream_handler {
    input_stream_handler: "SyncSetInputStreamHandler",
    options {
      [mediapipe.SyncSetInputStreamHandlerOptions.ext] {
        sync_set {
          tag_index: "LOOPBACK:0"
        }
      }
    }
  }
}`

	// GraphPBTxtGenerate defines the graph for text generation
	GraphPBTxtGenerate = `input_stream: "HTTP_REQUEST_PAYLOAD:input"
output_stream: "HTTP_RESPONSE_PAYLOAD:output"

node: {
  name: "LLMExecutor"
  calculator: "HttpLLMCalculator"
  input_stream: "LOOPBACK:loopback"
  input_stream: "HTTP_REQUEST_PAYLOAD:input"
  input_side_packet: "LLM_NODE_RESOURCES:llm"
  output_stream: "LOOPBACK:loopback"
  output_stream: "HTTP_RESPONSE_PAYLOAD:output"
  input_stream_info: {
    tag_index: 'LOOPBACK:0',
    back_edge: true
  }
  node_options: {
      [type.googleapis.com / mediapipe.LLMCalculatorOptions]: {
          models_path: "%s/models/%s",
          plugin_config: '{}',
          enable_prefix_caching: false,
          cache_size: 1,
          max_num_batched_tokens: 8192,
          dynamic_split_fuse: false, 
          max_num_seqs: 256,
          device: "GPU",
      }
  }
  input_stream_handler {
    input_stream_handler: "SyncSetInputStreamHandler",
    options {
      [mediapipe.SyncSetInputStreamHandlerOptions.ext] {
        sync_set {
          tag_index: "LOOPBACK:0"
        }
      }
    }
  }
}`

	// GraphPBTxtEmbed defines the graph for embeddings
	GraphPBTxtEmbed = `input_stream: "REQUEST_PAYLOAD:input"
output_stream: "RESPONSE_PAYLOAD:output"

node {
  name: "EmbeddingsExecutor"
  input_side_packet: "EMBEDDINGS_NODE_RESOURCES:embeddings_servable"
  calculator: "EmbeddingsCalculatorOV"
  input_stream: "REQUEST_PAYLOAD:input"
  output_stream: "RESPONSE_PAYLOAD:output"
  node_options: {
    [type.googleapis.com / mediapipe.EmbeddingsCalculatorOVOptions]: {
      models_path: "%s/models/%s",
      normalize_embeddings: true,
      pooling: CLS,
      truncate: true,
      target_device: "CPU"
    }
  }
}`

	// GraphPBTxtRerank defines the graph for reranking
	GraphPBTxtRerank = `input_stream: "REQUEST_PAYLOAD:input"
output_stream: "RESPONSE_PAYLOAD:output"

node {
  name: "RerankExecutor"
  input_side_packet: "RERANK_NODE_RESOURCES:rerank_servable"
  calculator: "RerankCalculatorOV"
  input_stream: "REQUEST_PAYLOAD:input"
  output_stream: "RESPONSE_PAYLOAD:output"
  node_options: {
    [type.googleapis.com / mediapipe.RerankCalculatorOVOptions]: {
      models_path: "%s/models/%s",
      target_device: "CPU"
    }
  }
}`

	// GraphPBTxtTextToImage defines the graph for text-to-image generation
	GraphPBTxtTextToImage = `input_stream: "HTTP_REQUEST_PAYLOAD:input"
output_stream: "HTTP_RESPONSE_PAYLOAD:output"

node: {
  name: "ImageGenExecutor"
  calculator: "ImageGenCalculator"
  input_stream: "HTTP_REQUEST_PAYLOAD:input"
  input_side_packet: "IMAGE_GEN_NODE_RESOURCES:pipes"
  output_stream: "HTTP_RESPONSE_PAYLOAD:output"
  node_options: {
    [type.googleapis.com / mediapipe.ImageGenCalculatorOptions]: {
      models_path: "./",
      device: "GPU",
      max_resolution: '2048x2048',
    }
  }
}`

	// GraphPBTxtSpeechToText defines the graph for speech recognition
	GraphPBTxtSpeechToText = `input_stream: "OVMS_PY_TENSOR:audio"
input_stream: "OVMS_PY_TENSOR_PARAMS:params"
output_stream: "OVMS_PY_TENSOR:result"

node {
  name: "%s"
  calculator: "PythonExecutorCalculator"
  input_side_packet: "PYTHON_NODE_RESOURCES:py"

  input_stream: "INPUT:audio"
  input_stream: "PARAMS:params"
  output_stream: "OUTPUT:result"
  node_options: {
    [type.googleapis.com/mediapipe.PythonExecutorCalculatorOptions]: {
      handler_path: "%s/scripts/speech-to-text/whisper.py"
    }
  }
}`

	// GraphPBTxtTextToSpeech defines the graph for text-to-speech
	GraphPBTxtTextToSpeech = `input_stream: "OVMS_PY_TENSOR:text"
input_stream: "OVMS_PY_TENSOR_VOICE:voice"
input_stream: "OVMS_PY_TENSOR_PARAMS:params"
output_stream: "OVMS_PY_TENSOR:audio"

node {
  name: "%s"
  calculator: "PythonExecutorCalculator"
  input_side_packet: "PYTHON_NODE_RESOURCES:py"

  input_stream: "INPUT:text"
  input_stream: "VOICE:voice"
  input_stream: "PARAMS:params"
  output_stream: "OUTPUT:audio"
  node_options: {
    [type.googleapis.com/mediapipe.PythonExecutorCalculatorOptions]: {
      handler_path: "%s/scripts/text-to-speech/text-to-speech.py"
    }
  }
}`

	// ChatTemplateJinja is the default chat template for models
	ChatTemplateJinja = `{%- if not tools is defined %}{% set tools = None %}{%- endif %}
{%- if not date_string is defined %}
    {%- if strftime_now is defined %}
        {%- set date_string = strftime_now("%Y-%m-%d") %}
    {%- else %}
        {%- set date_string = "2024-01-01" %}
    {%- endif %}
{%- endif %}
{# ==== 系统消息（可选性自定义） ==== #}
{%- if messages[0]['role'] == 'system' %}
    {%- set system_message = messages[0]['content'] | trim %}
    {%- set messages = messages[1:] %}
{%- else %}
    {%- set system_message = "你是一名可以调用工具的AI助手。" %}
{%- endif %}
<|start|>system
{{ system_message }}
知识截止: 2023-12-31
当前日期: {{ date_string }}
{%- if tools %}
可用工具:
{%- for t in tools %}
- {{ t | tojson(indent=4) }}
{%- endfor %}
调用工具时，请以 {"name": 工具名, "parameters": {...}} 的JSON严格返回。
{%- endif %}
<|end|>
{# ==== 聊天消息历史区块（无图片/无代码，纯文本） ==== #}
{%- for message in messages %}
    {%- if message['role'] in ['user', 'assistant'] %}
<|start|>{{ message['role'] }}
{{ message['content'] | trim }}
<|end|>
    {%- elif message['role'] in ['tool', 'function'] %}
<|start|>function
{%- if message['tool_call'] %}
{{ message['tool_call'] | tojson }}
{%- else %}
{{ message['content'] | tojson }}
{%- endif %}
<|end|>
    {%- endif %}
{%- endfor %}
{# ==== assistant新答复提示 ==== #}
{%- if add_generation_prompt %}
<|start|>assistant
{%- endif %}`
)

// InferToolParser infers the tool_parser value based on model name
func InferToolParser(modelName string) string {
	modelNameLower := strings.ToLower(modelName)

	// Check for specific model keywords
	if strings.Contains(modelNameLower, "qwen") {
		return "hermes3"
	}
	if strings.Contains(modelNameLower, "hermes") {
		return "hermes3"
	}
	if strings.Contains(modelNameLower, "llama") {
		return "llama3"
	}
	if strings.Contains(modelNameLower, "phi") {
		return "phi4"
	}
	if strings.Contains(modelNameLower, "mistral") {
		return "mistral"
	}

	// Default: empty string (no parser)
	return ""
}
