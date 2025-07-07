#*****************************************************************************
# Copyright 2025 Intel Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#*****************************************************************************

import os
import time
import requests
import subprocess
import threading
import http.server
import socketserver
import winreg
from urllib.parse import urlparse, parse_qs

WEB_SERVER_PORT = 5000
AOG_DOWNLOAD_URL = "https://smartvision-aipc-open.oss-cn-hangzhou.aliyuncs.com/aog/windows/aog.exe"
AOG_FOLDER = os.path.join(os.path.expanduser("~"), "AOG")
AOG_PATH = os.path.join(AOG_FOLDER, "aog.exe")

user_response = None


# 检查 AOG 服务器是否可用
def is_aog_available():
    try:
        response = requests.get("http://localhost:16688", timeout=3)
        return response.status_code == 200
    except requests.RequestException:
        return False


# 检查服务提供商是否存在
def get_service_provider():
    try:
        response = requests.get("http://127.0.0.1:16688/aog/v0.4/service_provider", timeout=3)
        return response.status_code == 200 and response.text.strip() != ""
    except requests.RequestException:
        return False


# 启动 Web 服务器，提供安装确认界面
class InstallPromptHandler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        global user_response
        parsed_path = urlparse(self.path)
        if parsed_path.path == "/install-prompt":
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            self.wfile.write(INSTALL_PROMPT_HTML.encode("utf-8"))
        elif parsed_path.path == "/user-response":
            query = parse_qs(parsed_path.query)
            user_response = query.get("choice", ["false"])[0] == "true"
            self.send_response(200)
            self.end_headers()


def start_web_server():
    server = socketserver.TCPServer(("0.0.0.0", WEB_SERVER_PORT), InstallPromptHandler)
    threading.Thread(target=server.serve_forever, daemon=True).start()


# 在浏览器中打开 URL
def open_browser(url):
    if os.name == "nt":
        os.system(f'start {url}')
    elif os.uname().sysname == "Darwin":
        os.system(f'open {url}')
    else:
        os.system(f'xdg-open {url}')


# 下载 AOG.exe
def download_aog():
    os.makedirs(AOG_FOLDER, exist_ok=True)
    try:
        response = requests.get(AOG_DOWNLOAD_URL, stream=True)
        with open(AOG_PATH, "wb") as f:
            for chunk in response.iter_content(1024):
                f.write(chunk)
        add_to_user_path(AOG_FOLDER) 
        return True
    except requests.RequestException:
        return False


# 启动 AOG 服务器
def install_aog():
    try:
        subprocess.Popen([AOG_PATH, "server", "start", "-d"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(3)  # 等待 AOG 启动
        return True
    except Exception:
        return False


# 导入 AOG 文件
def import_aog_file(aog_file_path):
    try:
        result = subprocess.run([AOG_PATH, "import", "--file", aog_file_path], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        return result.returncode == 0
    except Exception:
        return False
    

def add_to_user_path(dest_dir):
    """
    将指定目录添加到用户的环境变量 PATH 中。
    :param dest_dir: 要添加的目录路径
    :return: True 表示成功，False 表示失败
    """
    if os.name == "nt":  # Windows 平台
        try:
            reg_key = r"Environment"
            with winreg.OpenKey(winreg.HKEY_CURRENT_USER, reg_key, 0, winreg.KEY_READ) as key:
                try:
                    current_path, _ = winreg.QueryValueEx(key, "Path")
                except FileNotFoundError:
                    current_path = ""

            # 检查路径是否已存在
            paths = current_path.split(";") if current_path else []
            if dest_dir in paths:
                print("✅ 环境变量已存在")
                return True

            # 更新 PATH 值
            new_path = f"{current_path};{dest_dir}" if current_path else dest_dir
            with winreg.OpenKey(winreg.HKEY_CURRENT_USER, reg_key, 0, winreg.KEY_SET_VALUE) as key:
                winreg.SetValueEx(key, "Path", 0, winreg.REG_EXPAND_SZ, new_path)

            print("✅ 已添加到环境变量，请重新启动应用程序使更改生效")
            return True
        except Exception as e:
            print(f"❌ 添加环境变量失败: {e}")
            return False
    elif os.name == "posix":  # macOS 或 Linux 平台
        try:
            shell = os.environ.get("SHELL", "")
            shell_config_name = ".zshrc" if "zsh" in shell else ".bash_profile"
            shell_config_path = os.path.join(os.path.expanduser("~"), shell_config_name)
            export_line = f'export PATH="$PATH:{dest_dir}"\n'

            # 确保配置文件存在
            if not os.path.exists(shell_config_path):
                with open(shell_config_path, "w") as f:
                    f.write("")

            # 检查是否已存在路径
            with open(shell_config_path, "r") as f:
                content = f.read()
            if export_line in content:
                print("✅ 环境变量已存在")
                return True

            # 追加路径到配置文件
            with open(shell_config_path, "a") as f:
                f.write(f"\n{export_line}")
            print(f"✅ 已添加到 {shell_config_name}，请执行以下命令生效：\nsource {shell_config_path}")
            return True
        except Exception as e:
            print(f"❌ 添加环境变量失败: {e}")
            return False
    else:
        print("❌ 不支持的操作系统")
        return False


# HTML 页面
INSTALL_PROMPT_HTML = """
<html>
<body style="padding:20px;font-family:Arial">
    <h2>安装确认</h2>
    <p>需要安装 AOG 组件才能继续，是否允许？</p>
    <button onclick="respond(true)">同意安装</button>
    <button onclick="respond(false)">取消</button>
    <script>
        function respond(choice) {
            fetch('/user-response?choice=' + choice)
                .then(() => window.close());
        }
    </script>
</body>
</html>
"""


# 主入口
def AOGInit(aog_file_path=None):
    global user_response
    if aog_file_path is None:
        aog_file_path = os.path.join(os.getcwd(), ".aog")

    # 检查 AOG 是否可用
    if not is_aog_available():
        print("AOG 未运行，启动安装流程...")
        start_web_server()
        open_browser(f"http://localhost:{WEB_SERVER_PORT}/install-prompt")

        # 等待用户输入，最多 5 分钟
        start_time = time.time()
        while user_response is None and time.time() - start_time < 300:
            time.sleep(1)

        if not user_response:
            print("用户拒绝安装，退出。")
            return

        if not download_aog():
            print("AOG 下载失败，退出。")
            return

        if not install_aog():
            print("AOG 启动失败，退出。")
            return

    print("✅ AOG 已启动，检查服务提供商...")

    # 检查服务提供商
    if not get_service_provider():
        print("服务提供商不存在，尝试导入配置文件...")
        if import_aog_file(aog_file_path):
            print(f"✅ 成功导入 {aog_file_path}")
        else:
            print(f"❌ 导入失败: {aog_file_path}")
    else:
        print("✅ 服务提供商已存在，无需导入配置文件。")


# 让 Python 直接 `import aogchecker` 即可调用
if __name__ == "__main__":
    AOGInit()