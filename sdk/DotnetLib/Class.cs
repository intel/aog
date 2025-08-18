using System;
using System.Web;
using System.Diagnostics;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Win32;
using System.Net.Http;
using System.Text.Json;
using System.Net.WebSockets;
using System.Text;
using System.Threading.Channels;



namespace AOG
{
    public class AOGClient
    {
        private readonly HttpClient _client;
        private readonly string _baseUrl;

        public AOGClient(string version = "aog/v0.2")
        {
            if (!version.EndsWith("/")) version += "/";
            _baseUrl = $"http://127.0.0.1:16688/{version}";
            _client = new HttpClient { BaseAddress = new Uri(_baseUrl) };
        }

        // 通用请求方法
        private async Task<string> RequestAsync(HttpMethod method, string path, object? data = null)
        {
            try
            {
                if (path.StartsWith("/"))
                {
                    path = path.TrimStart('/');
                }

                HttpRequestMessage request = new HttpRequestMessage(method, path); // 使用相对路径

                if (data != null)
                {
                    var json = JsonSerializer.Serialize(data);
                    request.Content = new StringContent(json, System.Text.Encoding.UTF8, "application/json");
                }
                Console.WriteLine($"Request URL: {request.RequestUri}");
                Console.WriteLine($"Headers: {string.Join(", ", request.Headers)}");

                var response = await _client.SendAsync(request);
                response.EnsureSuccessStatusCode();

                return await response.Content.ReadAsStringAsync();
            }
            catch (Exception ex)
            {
                throw new Exception($"请求 {method} {path} 失败: {ex.Message}");
            }
        }

        // 获取服务
        public async Task<string> GetServicesAsync()
        {
            return await RequestAsync(HttpMethod.Get, "/service");
        }

        // 创建新服务
        public async Task<string> InstallServiceAsync(object data)
        {
            return await RequestAsync(HttpMethod.Post, "/service", data);
        }

        // 更新服务
        public async Task<string> UpdateServiceAsync(object data)
        {
            return await RequestAsync(HttpMethod.Put, "/service", data);
        }

        // 查看模型
        public async Task<string> GetModelsAsync()
        {
            return await RequestAsync(HttpMethod.Get, "/model");
        }

        // 安装模型
        public async Task<string> InstallModelAsync(object data)
        {
            return await RequestAsync(HttpMethod.Post, "/model", data);
        }

        // 流式安装模型
        public async Task InstallModelStreamAsync(
            object data,
            Action<JsonElement> onData,
            Action<string> onError,
            Action onEnd)
        {
            try
            {
                var json = JsonSerializer.Serialize(data);
                var content = new StringContent(json, System.Text.Encoding.UTF8, "application/json");
                var request = new HttpRequestMessage(HttpMethod.Post, $"{_baseUrl}model/stream")
                {
                    Content = content
                };

                var response = await _client.SendAsync(request, HttpCompletionOption.ResponseHeadersRead);
                response.EnsureSuccessStatusCode();

                using var stream = await response.Content.ReadAsStreamAsync();
                using var reader = new System.IO.StreamReader(stream);

                while (!reader.EndOfStream)
                {
                    var line = await reader.ReadLineAsync();
                    if (string.IsNullOrWhiteSpace(line))
                        continue;

                    try
                    {
                        string rawData = line.StartsWith("data:") ? line.Substring(5) : line;
                        var responseData = JsonSerializer.Deserialize<JsonElement>(rawData);

                        onData?.Invoke(responseData);

                        var status = responseData.GetProperty("status").GetString();
                        if (status == "success" || status == "error")
                        {
                            onEnd?.Invoke();
                            break;
                        }
                    }
                    catch (Exception ex)
                    {
                        onError?.Invoke($"解析流数据失败: {ex.Message}");
                    }
                }
            }
            catch (Exception ex)
            {
                onError?.Invoke($"流式安装模型失败: {ex.Message}");
            }
        }

        // 取消流式安装模型
        public async Task<string> CancelInstallModelAsync(object data)
        {
            return await RequestAsync(HttpMethod.Post, "/model/stream/cancel", data);
        }

        // 删除模型
        public async Task<string> DeleteModelAsync(object data)
        {
            return await RequestAsync(HttpMethod.Delete, "/model", data);
        }

        // 查看模型提供商
        public async Task<string> GetServiceProvidersAsync()
        {
            return await RequestAsync(HttpMethod.Get, "/service_provider");
        }

        // 新增模型提供商
        public async Task<string> AddServiceProviderAsync(object data)
        {
            return await RequestAsync(HttpMethod.Post, "/service_provider", data);
        }

        // 更新模型提供商
        public async Task<string> UpdateServiceProviderAsync(object data)
        {
            return await RequestAsync(HttpMethod.Put, "/service_provider", data);
        }

        // 删除模型提供商
        public async Task<string> DeleteServiceProviderAsync(object data)
        {
            return await RequestAsync(HttpMethod.Delete, "/service_provider", data);
        }

        // 获取模型列表
        public async Task<string> GetModelAvailiableAsync()
        {
            return await RequestAsync(HttpMethod.Get, "/services/models");
        }

        // 获取推荐模型列表
        public async Task<string> GetModelsRecommendedAsync()
        {
            return await RequestAsync(HttpMethod.Get, "/model/recommend");
        }

        // 获取支持模型列表
        public async Task<string> GetModelsSupportedAsync()
        {
            return await RequestAsync(HttpMethod.Get, "/model/support");
        }

        // 获取问学支持模型列表
        public async Task<string> GetSmartvisionModelsSupportedAsync(Dictionary<string, string> headers)
        {
            // 构建带查询参数的 URL
            var queryParams = HttpUtility.ParseQueryString(string.Empty);
            foreach (var header in headers)
            {
                if (header.Key.ToLower() == "env_type") // 忽略大小写
                {
                    queryParams[header.Key] = header.Value;
                }
            }
            string fullPath = $"/model/support/smartvision?{queryParams.ToString()}";

            return await RequestAsync(HttpMethod.Get, fullPath, null); // 不再传递 headers
        }

        // 导入配置文件
        public async Task<string> ImportConfigAsync(string filePath)
        {
            // 读取json文件内容
            string jsonContent = await File.ReadAllTextAsync(filePath);
            var jsonElement = JsonSerializer.Deserialize<JsonElement>(jsonContent);
            return await RequestAsync(HttpMethod.Post, "/config/import", jsonElement);
        }

        // 导出配置文件
        public async Task<string> ExportConfigAsync(object data)
        {
            try
            {
                // 调用 RequestAsync 获取配置文件的 JSON 响应
                var config = await RequestAsync(HttpMethod.Get, "/config/export", data);

                string userDirectory = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
                string aogDirectory = Path.Combine(userDirectory, "AOG");
                string aogFilePath = Path.Combine(aogDirectory, ".aog");

                await File.WriteAllTextAsync(aogFilePath, config);

                // 返回文件路径
                return aogFilePath;
            }
            catch (Exception ex)
            {
                throw new Exception($"导出配置文件失败: {ex.Message}");
            }
        }

        // Chat
        public async Task<string> ChatAsync(object data, bool isStream = false, Action<JsonElement>? onData = null, Action<string>? onError = null, Action? onEnd = null)
        {
            try
            {
                var json = JsonSerializer.Serialize(data);
                var content = new StringContent(json, System.Text.Encoding.UTF8, "application/json");

                if (isStream)
                {
                    var request = new HttpRequestMessage(HttpMethod.Post, $"{_baseUrl}services/chat")
                    {
                        Content = content
                    };

                    var response = await _client.SendAsync(request, HttpCompletionOption.ResponseHeadersRead);
                    response.EnsureSuccessStatusCode();

                    using var stream = await response.Content.ReadAsStreamAsync();
                    using var reader = new System.IO.StreamReader(stream);

                    while (!reader.EndOfStream)
                    {
                        var line = await reader.ReadLineAsync();
                        if (string.IsNullOrWhiteSpace(line))
                            continue;

                        try
                        {
                            string rawData = line.StartsWith("data:") ? line.Substring(5) : line;
                            var responseData = JsonSerializer.Deserialize<JsonElement>(rawData);

                            onData?.Invoke(responseData);

                            if (responseData.TryGetProperty("finished", out var finishedProperty) && finishedProperty.GetBoolean())
                            {
                                onEnd?.Invoke();
                                break;
                            }
                        }
                        catch (JsonException jsonEx)
                        {
                            onError?.Invoke($"JSON 解析错误: {jsonEx.Message}");
                        }
                        catch (Exception ex)
                        {
                            onError?.Invoke($"解析流数据失败: {ex.Message}");
                        }
                    }

                    return "Stream completed";
                }
                else
                {
                    var response = await _client.PostAsync("services/chat", content);
                    response.EnsureSuccessStatusCode();

                    return await response.Content.ReadAsStringAsync();
                }
            }
            catch (Exception ex)
            {
                throw new Exception($"Chat 服务请求失败: {ex.Message}");
            }
        }

        // Generate
        public async Task<string> GenerateAsync(object data, bool isStream = false, Action<JsonElement>? onData = null, Action<string>? onError = null, Action? onEnd = null)
        {
            try
            {
                var json = JsonSerializer.Serialize(data);
                var content = new StringContent(json, System.Text.Encoding.UTF8, "application/json");

                if (isStream)
                {
                    var request = new HttpRequestMessage(HttpMethod.Post, $"{_baseUrl}services/generate")
                    {
                        Content = content
                    };

                    var response = await _client.SendAsync(request, HttpCompletionOption.ResponseHeadersRead);
                    response.EnsureSuccessStatusCode();

                    using var stream = await response.Content.ReadAsStreamAsync();
                    using var reader = new System.IO.StreamReader(stream);

                    while (!reader.EndOfStream)
                    {
                        var line = await reader.ReadLineAsync();
                        if (string.IsNullOrWhiteSpace(line))
                            continue;

                        try
                        {
                            string rawData = line.StartsWith("data:") ? line.Substring(5) : line;
                            var responseData = JsonSerializer.Deserialize<JsonElement>(rawData);

                            onData?.Invoke(responseData);

                            var status = responseData.GetProperty("status").GetString();
                            if (status == "success" || status == "error")
                            {
                                onEnd?.Invoke();
                                break;
                            }
                        }
                        catch (Exception ex)
                        {
                            onError?.Invoke($"解析流数据失败: {ex.Message}");
                        }
                    }

                    return "Stream completed";
                }
                else
                {
                    var response = await _client.PostAsync("/services/generate", content);
                    response.EnsureSuccessStatusCode();

                    return await response.Content.ReadAsStringAsync();
                }
            }
            catch (Exception ex)
            {
                throw new Exception($"Generate 服务请求失败: {ex.Message}");
            }
        }

        // embed
        public async Task<string> EmbedAsync(object data)
        {
            return await RequestAsync(HttpMethod.Post, "/services/embed", data);
        }

        // text-to-image
        public async Task<string> TextToImageAsync(object data)
        {
            return await RequestAsync(HttpMethod.Post, "/services/text-to-image", data);
        }


        // speech-to-text-ws
        /// <summary>
        /// 语音流式识别，返回会话对象，支持事件回调和流式写入
        /// </summary>
        /// <param name="wsUrl">WebSocket服务地址</param>
        /// <param name="model">模型名</param>
        /// <param name="language">语言</param>
        /// <param name="sampleRate">采样率</param>
        /// <param name="channels">声道数</param>
        /// <param name="useVad">是否使用VAD</param>
        /// <param name="returnFormat">返回格式</param>
        public SpeechToTextStreamSession SpeechToTextStream(
            string wsUrl,
            string model,
            string language = "zh",
            int sampleRate = 16000,
            int channels = 1,
            bool useVad = true,
            string returnFormat = "text")
        {
            var session = new SpeechToTextStreamSession(model);
            _ = session.ConnectAsync(wsUrl, language, sampleRate, channels, useVad, returnFormat);
            return session;
        }


        // 检查 aog 状态
        public async Task<bool> IsAOGAvailiableAsync()
        {
            try
            {
                var response = await _client.GetAsync("/");
                return response.IsSuccessStatusCode;
            }
            catch (Exception ex)
            {
                throw new Exception($"检查 AOG 状态失败: {ex.Message}");
            }
        }

        // 检查 aog 是否下载
        public bool IsAOGExisted()
        {
            try
            {
                // 获取用户目录
                string userDirectory = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);

                // 根据操作系统设置路径
                string aogPath;
                if (OperatingSystem.IsWindows())
                {
                    aogPath = Path.Combine(userDirectory, "AOG", "aog.exe");
                }
                else if (OperatingSystem.IsMacOS())
                {
                    aogPath = Path.Combine(userDirectory, "AOG", "aog");
                }
                else
                {
                    throw new PlatformNotSupportedException("当前操作系统不支持");
                }

                // 检查文件是否存在
                return File.Exists(aogPath);
            }
            catch (Exception ex)
            {
                Console.WriteLine($"检查 AOG 文件失败: {ex.Message}");
                return false;
            }
        }

        // 下载 AOG
        public async Task<bool> DownloadAOGAsync()
        {
            try
            {
                // 根据操作系统选择下载 URL 和目标路径
                string url = OperatingSystem.IsMacOS()
                    ? "https://smartvision-aipc-open.oss-cn-hangzhou.aliyuncs.com/aog/macos/aog.zip"
                    : "https://smartvision-aipc-open.oss-cn-hangzhou.aliyuncs.com/aog/windows/aog.exe";

                string userDirectory = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
                string aogDirectory = Path.Combine(userDirectory, "AOG");
                string destFileName = OperatingSystem.IsMacOS() ? "aog.zip" : "aog.exe";
                string destFilePath = Path.Combine(aogDirectory, destFileName);

                // 创建 AOG 目录
                if (!Directory.Exists(aogDirectory))
                {
                    Directory.CreateDirectory(aogDirectory);
                }

                // 下载文件
                using var client = new HttpClient();
                using var response = await client.GetAsync(url, HttpCompletionOption.ResponseHeadersRead);
                response.EnsureSuccessStatusCode();

                using var fileStream = new FileStream(destFilePath, FileMode.Create, FileAccess.Write, FileShare.None);
                await response.Content.CopyToAsync(fileStream);

                Console.WriteLine($"✅ 下载完成: {destFilePath}");

                // 如果是 macOS，解压 ZIP 文件
                if (OperatingSystem.IsMacOS())
                {
                    string extractedPath = Path.Combine(aogDirectory, "aog");
                    System.IO.Compression.ZipFile.ExtractToDirectory(destFilePath, aogDirectory, true);
                    File.Delete(destFilePath);
                    Console.WriteLine($"✅ 解压完成: {extractedPath}");

                    // 设置可执行权限
                    if (File.Exists(extractedPath))
                    {
                        var chmod = new ProcessStartInfo
                        {
                            FileName = "chmod",
                            Arguments = "+x " + extractedPath,
                            RedirectStandardOutput = true,
                            RedirectStandardError = true,
                            UseShellExecute = false,
                            CreateNoWindow = true
                        };
                        Process.Start(chmod)?.WaitForExit();
                    }
                }

                // 添加 AOG 目录到环境变量
                AddToEnvironmentPath(aogDirectory);

                return true;
            }
            catch (Exception ex)
            {
                Console.WriteLine($"❌ 下载 AOG 失败: {ex.Message}");
                return false;
            }
        }

        // 将路径添加到环境变量
        private void AddToEnvironmentPath(string directory)
        {
            try
            {
                if (OperatingSystem.IsWindows())
                {
                    // Windows: 修改注册表以永久添加到 PATH
                    const string regKey = @"Environment";
                    using var key = Registry.CurrentUser.OpenSubKey(regKey, writable: true);
                    if (key == null) throw new Exception("无法打开注册表键");

                    string? currentPath = key.GetValue("Path", "", RegistryValueOptions.DoNotExpandEnvironmentNames)?.ToString();
                    if (currentPath == null || !currentPath.Contains(directory))
                    {
                        string newPath = string.IsNullOrEmpty(currentPath) ? directory : $"{currentPath};{directory}";
                        key.SetValue("Path", newPath, RegistryValueKind.ExpandString);
                        Console.WriteLine("✅ 已将 AOG 目录添加到环境变量 PATH");
                    }
                    else
                    {
                        Console.WriteLine("✅ AOG 目录已存在于环境变量 PATH 中");
                    }
                }
                else if (OperatingSystem.IsMacOS() || OperatingSystem.IsLinux())
                {
                    // macOS/Linux: 修改 shell 配置文件
                    string shellConfigPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".zshrc");
                    if (!File.Exists(shellConfigPath))
                    {
                        shellConfigPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".bashrc");
                    }

                    string exportLine = $"export PATH=\"$PATH:{directory}\"";
                    if (File.Exists(shellConfigPath))
                    {
                        string content = File.ReadAllText(shellConfigPath);
                        if (!content.Contains(exportLine))
                        {
                            File.AppendAllText(shellConfigPath, Environment.NewLine + exportLine);
                            Console.WriteLine($"✅ 已将 AOG 目录添加到 {Path.GetFileName(shellConfigPath)}，请执行以下命令使其生效：\nsource {shellConfigPath}");
                        }
                        else
                        {
                            Console.WriteLine($"✅ AOG 目录已存在于 {Path.GetFileName(shellConfigPath)} 中");
                        }
                    }
                    else
                    {
                        File.WriteAllText(shellConfigPath, exportLine + Environment.NewLine);
                        Console.WriteLine($"✅ 已创建 {Path.GetFileName(shellConfigPath)} 并添加 AOG 目录，请执行以下命令使其生效：\nsource {shellConfigPath}");
                    }
                }
                else
                {
                    throw new PlatformNotSupportedException("当前操作系统不支持添加环境变量");
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"❌ 添加 AOG 目录到环境变量失败: {ex.Message}");
            }
        }

        // 启动 AOG 服务
        public bool InstallAOG()
        {
            try
            {
                string userDirectory = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
                string aogDirectory = Path.Combine(userDirectory, "AOG");
                string aogExecutable = OperatingSystem.IsMacOS()
                    ? Path.Combine(aogDirectory, "aog")
                    : Path.Combine(aogDirectory, "aog.exe");

                if (!File.Exists(aogExecutable))
                {
                    Console.WriteLine("❌ AOG 可执行文件不存在，请先下载。");
                    return false;
                }

                // 确保 PATH 包含 AOG 目录
                string pathEnv = Environment.GetEnvironmentVariable("PATH") ?? string.Empty;
                if (!pathEnv.Contains(aogDirectory))
                {
                    Environment.SetEnvironmentVariable("PATH", pathEnv + Path.PathSeparator + aogDirectory);
                }

                // 启动 AOG 服务
                var processStartInfo = new ProcessStartInfo
                {
                    FileName = aogExecutable,
                    Arguments = "server start -d",
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false,
                    CreateNoWindow = true
                };

                using var process = Process.Start(processStartInfo);
                if (process == null)
                {
                    Console.WriteLine("❌ 启动 AOG 服务失败。");
                    return false;
                }

                process.WaitForExit();

                if (process.ExitCode == 0)
                {
                    Console.WriteLine("✅ AOG 服务已启动。");
                    return true;
                }
                else
                {
                    Console.WriteLine($"❌ AOG 服务启动失败，退出码: {process.ExitCode}");
                    return false;
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"❌ 启动 AOG 服务失败: {ex.Message}");
                return false;
            }
        }


    }
        public class SpeechToTextStreamSession
    {
        private readonly ClientWebSocket _ws = new();
        private readonly Channel<byte[]> _audioQueue = Channel.CreateUnbounded<byte[]>();
        private bool _isTaskStarted = false;
        private string? _taskId;
        private readonly string _model;

        public event Action? OnOpen;
        public event Action<string>? OnTaskStarted;
        public event Action<string, string>? OnFinished;
        public event Action<string>? OnError;
        public event Action? OnClose;

        public SpeechToTextStreamSession(string model)
        {
            _model = model;
        }

        public async Task ConnectAsync(string wsUrl, string language = "zh", int sampleRate = 16000, int channels = 1, bool useVad = true, string returnFormat = "text")
        {
            await _ws.ConnectAsync(new Uri(wsUrl), CancellationToken.None);
            OnOpen?.Invoke();

            // 发送 run-task 指令
            var runTask = new
            {
                task = "speech-to-text-ws",
                action = "run-task",
                model = _model,
                parameters = new
                {
                    format = "pcm",
                    sample_rate = sampleRate,
                    language,
                    use_vad = useVad,
                    return_format = returnFormat,
                    channels
                }
            };
            var msg = JsonSerializer.Serialize(runTask);
            await _ws.SendAsync(Encoding.UTF8.GetBytes(msg), WebSocketMessageType.Text, true, CancellationToken.None);

            _ = Task.Run(ReceiveLoop);
            _ = Task.Run(SendAudioLoop);
        }

        public async Task WriteAsync(byte[] audioChunk)
        {
            await _audioQueue.Writer.WriteAsync(audioChunk);
        }

        public async Task EndAsync()
        {
            if (_isTaskStarted && _taskId != null)
            {
                var finishTask = new
                {
                    task = "speech-to-text-ws",
                    action = "finish-task",
                    task_id = _taskId,
                    model = _model
                };
                var msg = JsonSerializer.Serialize(finishTask);
                await _ws.SendAsync(Encoding.UTF8.GetBytes(msg), WebSocketMessageType.Text, true, CancellationToken.None);
            }
            else
            {
                OnError?.Invoke("无法结束任务: 任务尚未启动或taskId为空");
            }
        }

        private async Task SendAudioLoop()
        {
            await foreach (var chunk in _audioQueue.Reader.ReadAllAsync())
            {
                if (_isTaskStarted)
                {
                    await _ws.SendAsync(chunk, WebSocketMessageType.Binary, true, CancellationToken.None);
                }
            }
        }

        private async Task ReceiveLoop()
        {
            var buffer = new byte[4096];
            while (_ws.State == WebSocketState.Open)
            {
                var result = await _ws.ReceiveAsync(buffer, CancellationToken.None);
                if (result.MessageType == WebSocketMessageType.Close)
                {
                    OnClose?.Invoke();
                    break;
                }
                var msg = Encoding.UTF8.GetString(buffer, 0, result.Count);
                try
                {
                    using var doc = JsonDocument.Parse(msg);
                    var root = doc.RootElement;
                    var header = root.GetProperty("header");
                    var evt = header.GetProperty("event").GetString();
                    switch (evt)
                    {
                        case "task-started":
                            _taskId = header.GetProperty("task_id").GetString();
                            _isTaskStarted = true;
                            OnTaskStarted?.Invoke(_taskId!);
                            break;
                        case "task-finished":
                            var text = root.TryGetProperty("text", out var t) ? t.GetString() : "";
                            var taskId = root.TryGetProperty("task_id", out var tid) ? tid.GetString() : "";
                            OnFinished?.Invoke(text ?? "", taskId ?? "");
                            await _ws.CloseAsync(WebSocketCloseStatus.NormalClosure, "done", CancellationToken.None);
                            break;
                        case "error":
                            var errMsg = root.TryGetProperty("message", out var em) ? em.GetString() : "服务器返回错误";
                            OnError?.Invoke(errMsg ?? "服务器返回错误");
                            break;
                        default:
                            // 其它事件
                            break;
                    }
                }
                catch (Exception ex)
                {
                    OnError?.Invoke($"消息解析失败: {ex.Message}");
                }
            }
        }
    }

}
