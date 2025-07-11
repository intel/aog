@echo off
REM *****************************************************************************
REM Copyright 2025 Intel Corporation
REM
REM Licensed under the Apache License, Version 2.0 (the "License");
REM you may not use this file except in compliance with the License.
REM You may obtain a copy of the License at
REM
REM     http://www.apache.org/licenses/LICENSE-2.0
REM
REM Unless required by applicable law or agreed to in writing, software
REM distributed under the License is distributed on an "AS IS" BASIS,
REM WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
REM See the License for the specific language governing permissions and
REM limitations under the License.
REM *****************************************************************************

set "USER_HOME=%USERPROFILE%"
set "AOG_HOME=%USER_HOME%\AOG"
set "PATH=%AOG_HOME%;%PATH%"

REM 使用 start 命令独立启动 aog，不依赖父进程
start "" /b aog server start -d
