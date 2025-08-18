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

package aogchecker

/*
#include <stdlib.h>
*/
import "C"

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	userResponse   chan bool
	webServerMutex sync.Mutex
	webServerPort  = 5000
)

//export AOGInit
func AOGInit(aogFilePath *C.char) {
	var configPath string
	if aogFilePath != nil {
		configPath = C.GoString(aogFilePath)
	} else {
		configPath = filepath.Join(getProjectRootDirectory(), ".aog")
	}

	// 检查 AOG 是否可用
	if !isAOGAvailable() {
		fmt.Println("AOG 不可用，启动 Web 服务器等待用户确认...")
		userResponse = make(chan bool)
		startWebServer()

		if !openBrowser(fmt.Sprintf("http://localhost:%d/install-prompt", webServerPort)) {
			fmt.Println("无法打开浏览器。")
			return
		}

		select {
		case choice := <-userResponse:
			if !choice {
				fmt.Println("用户取消了安装 AOG。")
				return
			}

			// 下载并安装 AOG
			if !downloadAOG() {
				fmt.Println("下载 AOG 失败。")
				return
			}

			if !installAOG() {
				fmt.Println("安装 AOG 失败。")
				return
			}
		case <-time.After(5 * time.Minute):
			fmt.Println("等待超时，未安装 AOG。")
			return
		}
	}

	fmt.Println("✅ AOG 已启动，检查服务提供商...")

	// 检查服务提供商
	if !getServiceProvider() {
		fmt.Println("服务提供商不存在，尝试导入配置文件...")
		if importAOG(configPath) {
			fmt.Printf("✅ 成功导入配置文件: %s\n", configPath)
		} else {
			fmt.Printf("❌ 导入配置文件失败: %s\n", configPath)
		}
	} else {
		fmt.Println("✅ 服务提供商已存在，无需导入配置文件。")
	}
}

func main() {} // 必须保留空的 main 函数

// ========== 核心逻辑实现 ==========

func isAOGAvailable() bool {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost:16688")
	return err == nil && resp.StatusCode == http.StatusOK
}

func getServiceProvider() bool {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:16688/aog/v0.2/service_provider")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return err == nil && len(body) > 0
}

func downloadAOG() bool {
	url := "https://smartvision-aipc-open.oss-cn-hangzhou.aliyuncs.com/aog/windows/aog.exe"
	userDir, _ := os.UserHomeDir()
	dest := filepath.Join(userDir, "AOG", "aog.exe")

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false
	}

	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	out, err := os.Create(dest)
	if err != nil {
		return false
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)

	sucess := addToUserPathUnix(filepath.Join(userDir, "AOG"))
	if !sucess {
		fmt.Println("❌ 添加到环境变量失败")
	}
	return err == nil
}

func installAOG() bool {
	userDir, _ := os.UserHomeDir()
	aogPath := filepath.Join(userDir, "AOG", "aog.exe")

	cmd := exec.Command(aogPath, "server", "start", "-d")
	return cmd.Start() == nil
}

func importAOG(configPath string) bool {
	userDir, _ := os.UserHomeDir()
	aogPath := filepath.Join(userDir, "AOG", "aog.exe")

	if !fileExists(configPath) {
		log.Fatalf("配置文件未找到: %s", configPath)
	}

	cmd := exec.Command(aogPath, "import", "--file", configPath)
	return cmd.Run() == nil
}

// ========== 系统交互部分 ==========

func startWebServer() {
	webServerMutex.Lock()
	defer webServerMutex.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/install-prompt", handleInstallPrompt)
	mux.HandleFunc("/user-response", handleUserResponse)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", webServerPort))
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		log.Fatal(http.Serve(listener, mux))
	}()
}

func handleInstallPrompt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, installPromptHTML)
}

func handleUserResponse(w http.ResponseWriter, r *http.Request) {
	choice := r.URL.Query().Get("choice") == "true"
	userResponse <- choice
	w.WriteHeader(http.StatusOK)
}

func openBrowser(url string) bool {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start() == nil
}

// ========== 工具方法 ==========

func getProjectRootDirectory() string {
	return filepath.Dir(os.Args[0])
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	return err == nil && !info.IsDir()
}

func AddToUserPath(destDir string) bool {
	if runtime.GOOS == "windows" {
		return addToUserPathWindows(destDir)
	} else if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		return addToUserPathUnix(destDir)
	} else {
		fmt.Println("❌ 不支持的操作系统")
		return false
	}
}

func addToUserPathWindows(destDir string) bool {
	cmd := exec.Command("reg", "query", `HKCU\Environment`, "/v", "Path")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("❌ 无法查询注册表:", err)
		return false
	}

	currentPath := ""
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Path") {
			parts := strings.Split(line, "    ")
			if len(parts) > 1 {
				currentPath = strings.TrimSpace(parts[len(parts)-1])
			}
			break
		}
	}

	// 检查路径是否已存在
	paths := strings.Split(currentPath, ";")
	for _, p := range paths {
		if strings.EqualFold(p, destDir) {
			fmt.Println("✅ 环境变量已存在")
			return true
		}
	}

	// 更新 PATH 值
	newPath := currentPath
	if currentPath != "" {
		newPath += ";"
	}
	newPath += destDir

	cmd = exec.Command("reg", "add", `HKCU\Environment`, "/v", "Path", "/t", "REG_EXPAND_SZ", "/d", newPath, "/f")
	err = cmd.Run()
	if err != nil {
		fmt.Println("❌ 添加环境变量失败:", err)
		return false
	}

	fmt.Println("✅ 已添加到环境变量，请重新启动应用程序使更改生效")
	return true
}

func addToUserPathUnix(destDir string) bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("❌ 无法获取用户主目录:", err)
		return false
	}

	shell := os.Getenv("SHELL")
	shellConfigName := ".bash_profile"
	if strings.Contains(shell, "zsh") {
		shellConfigName = ".zshrc"
	}
	shellConfigPath := filepath.Join(homeDir, shellConfigName)
	exportLine := fmt.Sprintf(`export PATH="$PATH:%s"`, destDir)

	// 确保配置文件存在
	if _, err := os.Stat(shellConfigPath); os.IsNotExist(err) {
		file, err := os.Create(shellConfigPath)
		if err != nil {
			fmt.Println("❌ 无法创建配置文件:", err)
			return false
		}
		file.Close()
	}

	// 检查是否已存在路径
	content, err := os.ReadFile(shellConfigPath)
	if err != nil {
		fmt.Println("❌ 无法读取配置文件:", err)
		return false
	}
	if strings.Contains(string(content), exportLine) {
		fmt.Println("✅ 环境变量已存在")
		return true
	}

	// 追加路径到配置文件
	file, err := os.OpenFile(shellConfigPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Println("❌ 无法打开配置文件:", err)
		return false
	}
	defer file.Close()

	_, err = file.WriteString("\n" + exportLine + "\n")
	if err != nil {
		fmt.Println("❌ 无法写入配置文件:", err)
		return false
	}

	fmt.Printf("✅ 已添加到 %s，请执行以下命令生效：\nsource %s\n", shellConfigName, shellConfigPath)
	return true
}

// ========== HTML模板 ==========

const installPromptHTML = `
<html><body style="padding:20px;font-family:Arial">
    <h2>安装确认</h2>
    <p>需要安装AOG组件才能继续，是否允许？</p>
    <button onclick="respond(true)">同意安装</button>
    <button onclick="respond(false)">取消</button>
    <script>
        function respond(choice) {
            fetch('/user-response?choice=' + choice)
                .then(() => window.close())
        }
    </script>
</body></html>`
