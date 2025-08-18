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
using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Hosting;

namespace AogCheckerLib
{
    public class AogChecker
    {
        private static readonly int WebServerPort = 5000;
        private static readonly string AogUrl = Environment.GetEnvironmentVariable("AOG_URL") ?? "https://smartvision-aipc-open.oss-cn-hangzhou.aliyuncs.com/aog/windows/aog.exe";
        private static readonly HttpClient HttpClient = new HttpClient();
        private static TaskCompletionSource<bool>? userResponse;

        public static async Task AOGInit(string? aogFilePath = null)
        {
            userResponse = new TaskCompletionSource<bool>();
            string configPath = aogFilePath ?? Path.Combine(AppContext.BaseDirectory, ".aog");

            // 检查 AOG 是否可用
            if (!await IsAOGAvailable())
            {
                Console.WriteLine("AOG 不可用，启动 Web 服务器等待用户确认...");
                using var cts = new CancellationTokenSource(TimeSpan.FromMinutes(5));
                _ = StartWebServer();
                OpenBrowser($"http://localhost:{WebServerPort}/install-prompt");

                try
                {
                    // 等待用户响应或超时
                    bool choice = await Task.WhenAny(userResponse.Task, Task.Delay(Timeout.Infinite, cts.Token)) == userResponse.Task && userResponse.Task.Result;
                    if (!choice)
                    {
                        Console.WriteLine("用户取消了安装 AOG。");
                        return;
                    }

                    // 下载并安装 AOG
                    if (!await DownloadAOG())
                    {
                        Console.WriteLine("下载 AOG 失败。");
                        return;
                    }

                    if (!InstallAOG())
                    {
                        Console.WriteLine("安装 AOG 失败。");
                        return;
                    }
                }
                catch (OperationCanceledException)
                {
                    Console.WriteLine("等待超时，未安装 AOG。");
                    return;
                }
            }

            Console.WriteLine("✅ AOG 已启动，检查服务提供商...");

            // 检查服务提供商
            if (!await GetServiceProvider())
            {
                Console.WriteLine("服务提供商不存在，尝试导入配置文件...");
                if (ImportAOG(configPath))
                {
                    Console.WriteLine($"✅ 成功导入配置文件: {configPath}");
                }
                else
                {
                    Console.WriteLine($"❌ 导入配置文件失败: {configPath}");
                }
            }
            else
            {
                Console.WriteLine("✅ 服务提供商已存在，无需导入配置文件。");
            }
        }

        private static async Task<bool> IsAOGAvailable()
        {
            try
            {
                var response = await HttpClient.GetAsync("http://localhost:16688");
                return response.IsSuccessStatusCode;
            }
            catch
            {
                return false;
            }
        }

        private static async Task StartWebServer()
        {
            var builder = WebApplication.CreateBuilder();
            var app = builder.Build();

            app.MapGet("/install-prompt", async context =>
            {
                await context.Response.WriteAsync("<html><body><h2>安装确认</h2><p>需要安装 AOG 组件才能继续，是否允许？</p>"
                    + "<button onclick=\"respond(true)\">同意安装</button>"
                    + "<button onclick=\"respond(false)\">取消</button>"
                    + "<script>function respond(choice) { fetch('/user-response?choice=' + choice).then(() => window.close()) }</script>"
                    + "</body></html>");
            });

            app.MapGet("/user-response", async context =>
            {
                var choice = context.Request.Query["choice"] == "true";
                userResponse?.TrySetResult(choice);
                await context.Response.WriteAsync("OK");
            });

            await app.RunAsync($"http://localhost:{WebServerPort}");
        }

        private static void OpenBrowser(string url)
        {
            try
            {
                string cmd = Environment.OSVersion.Platform switch
                {
                    PlatformID.Win32NT => "cmd",
                    PlatformID.MacOSX => "open",
                    _ => "xdg-open"
                };

                Process.Start(new ProcessStartInfo
                {
                    FileName = cmd,
                    Arguments = $"/c start {url}",
                    CreateNoWindow = true
                });
            }
            catch (Exception ex)
            {
                Console.WriteLine($"打开浏览器失败: {ex.Message}");
            }
        }

        private static async Task<bool> DownloadAOG()
        {
            string dest = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), "AOG", "aog.exe");
            Directory.CreateDirectory(Path.GetDirectoryName(dest)!);

            try
            {
                var bytes = await HttpClient.GetByteArrayAsync(AogUrl);
                await File.WriteAllBytesAsync(dest, bytes);
                await AddToUserPathWindows(Path.GetDirectoryName(dest)!);
                return true;
            }
            catch (Exception ex)
            {
                Console.WriteLine($"下载 AOG 失败: {ex.Message}");
                return false;
            }
        }

        private static bool InstallAOG()
        {
            try
            {
                string aogPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), "AOG", "aog.exe");
                var process = Process.Start(aogPath, "server start -d");
                return process != null;
            }
            catch (Exception ex)
            {
                Console.WriteLine($"安装 AOG 失败: {ex.Message}");
                return false;
            }
        }

        private static async Task<bool> GetServiceProvider()
        {
            try
            {
                var response = await HttpClient.GetAsync("http://127.0.0.1:16688/aog/v0.2/service_provider");
                var content = await response.Content.ReadAsStringAsync();
                return !string.IsNullOrEmpty(content);
            }
            catch (Exception ex)
            {
                Console.WriteLine($"❌ 获取服务提供商失败: {ex.Message}");
                return false;
            }
        }

        private static bool ImportAOG(string configPath)
        {
            try
            {
                if (!File.Exists(configPath)) throw new FileNotFoundException("配置文件未找到", configPath);

                string aogPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), "AOG", "aog.exe");
                var process = Process.Start(aogPath, $"import --file {configPath}");
                return process != null && process.WaitForExit(5000);
            }
            catch (Exception ex)
            {
                Console.WriteLine($"导入 AOG 配置失败: {ex.Message}");
                return false;
            }
        }

        private static bool AddToUserPathWindows(string destDir)
        {
            try
            {
                const string regKey = @"Environment";
                using (var key = Registry.CurrentUser.OpenSubKey(regKey, writable: true))
                {
                    if (key == null)
                    {
                        Console.WriteLine("❌ 无法访问注册表键");
                        return false;
                    }

                    var currentPath = key.GetValue("Path") as string ?? string.Empty;

                    // 检查路径是否已存在
                    var paths = currentPath.Split(';', StringSplitOptions.RemoveEmptyEntries);
                    if (Array.Exists(paths, path => path.Equals(destDir, StringComparison.OrdinalIgnoreCase)))
                    {
                        Console.WriteLine("✅ 环境变量已存在");
                        return true;
                    }

                    // 更新 PATH 值
                    var newPath = string.IsNullOrEmpty(currentPath) ? destDir : $"{currentPath};{destDir}";
                    key.SetValue("Path", newPath, RegistryValueKind.ExpandString);

                    Console.WriteLine("✅ 已添加到环境变量，请重新启动应用程序使更改生效");
                    return true;
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"❌ 添加环境变量失败: {ex.Message}");
                return false;
            }
        }
    }
}