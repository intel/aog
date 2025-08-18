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

﻿using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;
//using Newtonsoft.Json;
//using Newtonsoft.Json.Linq;

namespace Core
{
    public class TextToImageRequest
    {
        public string? Model { get; set; }
        public bool? Stream { get; set; }
        public InputData Input { get; set; } = new InputData();

        public class InputData
        {
            public string Prompt { get; set; } = string.Empty;
        }
    }
    public class TextToImageResponse
    {
        public ResponseData? Data { get; set; }
        public string? Id { get; set; }

        public class ResponseData
        {
            public string? Url { get; set; }
        }
    }

    // Get and Parse AI response
    public class TextToImageClient
    {
        private readonly HttpClient _httpClient;
        public string ModelName { get; set; } = "wanx2.1-t2i-turbo";
        private const string ApiUrl = "https://127.0.0.1:16688/aog/v0.2/services/text-to-image";
        private const string ApiKey = "your-api-key-here";

        public TextToImageClient()
        {
            _httpClient = new HttpClient();
            _httpClient.DefaultRequestHeaders.Add("Authorization", $"Bearer {ApiKey}");
        }
        public async Task<TextToImageResponse> GenerateImageAsync(string prompt)
        {
            var request = new TextToImageRequest
            {
                Model = ModelName,
                Input = new TextToImageRequest.InputData
                {
                    Prompt = prompt
                }
            };

            var json = JsonSerializer.Serialize(request);
            var content = new StringContent(json, Encoding.UTF8, "application/json");

            var response = await _httpClient.PostAsync(ApiUrl, content);
            response.EnsureSuccessStatusCode();

            var responseJson = await response.Content.ReadAsStringAsync();
            return JsonSerializer.Deserialize<TextToImageResponse>(responseJson)
                ?? throw new Exception("Failed to deserialize response");
        }
    }
}