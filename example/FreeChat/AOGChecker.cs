﻿using Newtonsoft.Json.Linq;
using System;
using System.Collections.Generic;
using System.Linq;
using System.Net;
using System.Text;
using System.Threading.Tasks;

namespace aog_checker_0
{
    public class AOGChecker
    {

        public class ComponentStatus
        {
            public List<JObject> MissingServices { get; set; }
            public List<JObject> MissingModels { get; set; }
            public List<JObject> UnhealthyComponents { get; set; }
            public ComponentStatus()
            {
                MissingServices = new List<JObject>();
                MissingModels = new List<JObject>();
                UnhealthyComponents = new List<JObject>();
            }
        }

        public static ComponentStatus? LastCheckResult { get; private set; }

        // Initialize AOG
        public static async Task AOGInit(Page page)
        {
            // create a log.txt
            string userFolder = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
            string logPath = Path.Combine(userFolder, "aog_log.txt");
            File.WriteAllText(logPath, "AOG 初始化开始\n");

            var aogAvailable = await CheckAogAvailabilityAsync(page);
            if (!aogAvailable)
            {
                return;
            }

            string projectRoot = GetProjectRootDirectory();
            string configPath = Path.Combine(projectRoot, ".aog");

            if (!File.Exists(configPath))
            {
                File.AppendAllText(logPath, ".aog 配置文件未找到，请将其放在项目根目录{configPath}。\n");
                throw new FileNotFoundException($".aog 配置文件未找到，请将其放在项目根目录{configPath}。");
            }

            // Download conponents need
            ExecuteCommand($"aog import --file {configPath}");

            File.AppendAllText(logPath, "AOGInit完成\n");
        }

        private static string GetProjectRootDirectory()
        {
            return AppDomain.CurrentDomain.BaseDirectory;
        }

        // Check whether AOG is available
        public static async Task<bool> CheckAogAvailabilityAsync(Page page)
        {
            // Change
            bool isAvailable = false;
            string userFolder = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
            string logPath = Path.Combine(userFolder, "aog_log.txt");
            File.AppendAllText(logPath, "CheckAogAvailabilityAsync\n");
            try
            {
                using (var client = new HttpClient())
                {
                    var checkResponse = await client.GetAsync("http://localhost:16688");
                    isAvailable = checkResponse.IsSuccessStatusCode;
                }
            }
            catch
            {
            }
            File.AppendAllText(logPath, "CheckAogAvailabilityAsync开始弹窗询问\n");
            if (!isAvailable)
            {
                var result = await page.DisplayAlert("AOG 服务不可用", "是否下载并安装 AOG？", "是", "否");
                if (result)
                {
                    await DownloadAogAsync();
                    await page.DisplayAlert("安装完成", "AOG 已下载并安装，请重启应用程序。", "确定");
                    return false;
                }
                else
                {
                    await page.DisplayAlert("错误", "AOG 服务不可用，用户取消安装。", "确定");
                    return false;
                }
            }
            return true;
        }


        // Download AOG
        public static async Task DownloadAogAsync()
        {
            string downloadUrl = "http://120.232.136.73:31619/aogdev/aog.exe";
            string userFolder = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
            string aogFolder = Path.Combine(userFolder, "AOG");

            if(!Directory.Exists(aogFolder))
            {
                Directory.CreateDirectory(aogFolder);
            }
            string destinationPath = Path.Combine(aogFolder, "aog.exe");
            using (var client = new HttpClient())
            {
                var response = await client.GetAsync(downloadUrl);
                response.EnsureSuccessStatusCode();
                var fileBytes = await response.Content.ReadAsByteArrayAsync();
                await File.WriteAllBytesAsync(destinationPath, fileBytes);
            }

            string logPath = Path.Combine(userFolder, "aog_log.txt");
            AddToEnvironmentVariable("PATH", aogFolder);

            ExecuteCommand("aog server start -d");

            File.AppendAllText(logPath, "DownloadAogAsync执行完成\n");
        }

        private static void ExecuteCommand(string command)
        {
            string userFolder = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
            string aogFolder = Path.Combine(userFolder, "AOG");
            string logPath = Path.Combine(userFolder, "aog_log.txt");
            var processInfo = new System.Diagnostics.ProcessStartInfo("cmd.exe", "/c " + command)
            {
                CreateNoWindow = true,
                UseShellExecute = false,
                RedirectStandardError = true,
                RedirectStandardOutput = true
            };

            using (var process = new System.Diagnostics.Process())
            {
                process.StartInfo = processInfo;
                process.Start();

                process.OutputDataReceived += (object sender, System.Diagnostics.DataReceivedEventArgs e) =>
                {
                    Console.WriteLine("output>>" + e.Data);
                    File.AppendAllText(logPath, "output>>" + e.Data);
                };
                process.BeginOutputReadLine();

                process.ErrorDataReceived += (object sender, System.Diagnostics.DataReceivedEventArgs e) =>
                {
                    Console.WriteLine("error>>" + e.Data);
                    File.AppendAllText(logPath, "error>>" + e.Data);
                };
                process.BeginErrorReadLine();

                process.WaitForExit();
                
                Console.WriteLine("ExitCode: {0}", process.ExitCode);
                File.AppendAllText(logPath, "ExitCode: {0}"+ process.ExitCode);
                process.Close();
            }
        }

        // Add oag path to environment variable
        private static void AddToEnvironmentVariable(string variable, string value)
        {
            string currentValue = Environment.GetEnvironmentVariable(variable, EnvironmentVariableTarget.User);
            if (!currentValue.Contains(value))
            {
                Environment.SetEnvironmentVariable(variable, currentValue + ";" + value, EnvironmentVariableTarget.User);
            }
        }
    }
}
