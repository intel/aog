===========================================
AOG Speech相关服务
===========================================

.. toctree::
   :maxdepth: 2
   :hidden:

本页面包含以下服务的详细规范：

* :ref:`speech_to_text_service`
* :ref:`speech_to_text_ws_service`

.. _speech_to_text_service:

Speech-To-Text 服务
=====================

.. _`custom_properties_speech_to_text`:

Custom Properties of its Service Providers
--------------------------------------------

除了在 :ref:`Metadata of AOG Service
Provider` 中定义的常见属性外, 语音识别服务提供商还可以将以下属性放入服务提供商元数据的 ``custom_properties`` 字段中。

.. list-table::
   :header-rows: 1

   * - 自定义属性
     - 值
     - 描述
   * - language
     - string
     - 指定音频内容的语言，如"zh"、"en"等

请求格式
--------------------------------------------

.. _`header_speech-to-text`:

请求头
___________

参见 :ref:`Common Fields in Header of Request`


.. _`request_speech-to-text`:

请求
______________

除了在 :ref:`Common Fields in Request Body` 中定义的字段外，服务在其请求 JSON 体中也可能包含以下字段：

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 附加 JSON 字段
     - 值
     - 是否必需
     - 描述
   * - audio
     - string
     - 必需
     - 音频文件的base64编码数据
   * - language
     - string
     - 可选
     - 音频内容的语言代码，如"zh"、"en"

.. _`response_speech-to-text`:

响应格式
--------------------------------------------

除了在 :ref:`Common Fields in Response Body` 中定义的字段外，该服务在其响应 JSON 体中可能还有以下字段：

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 附加 JSON 字段
     - 值
     - 是否必需
     - 描述
   * - segments
     - array
     - 必填
     - 识别结果的时间轴分段数组
   * - segments[].id
     - integer
     - 必填
     - 分段序号
   * - segments[].start
     - string
     - 必填
     - 分段开始时间
   * - segments[].end
     - string
     - 必填
     - 分段结束时间
   * - segments[].text
     - string
     - 必填
     - 分段文本内容

示例
--------------

发送请求

.. code-block:: shell

    curl https://localhost:16688/aog/v0.4/services/speech-to-text\
    -H "Content-Type: application/json" \
    -d '{
            "model": "NamoLi/whisper-large-v3-ov",
            "audio": "base64编码的音频数据",
            "language": "zh"
        }'

返回响应

.. code-block:: json

    {
        "data": {
            "segments": [
                {
                    "id": 0,
                    "start": "00:00:00.000",
                    "end": "00:00:03.500",
                    "text": "第一段识别的文本内容"
                },
                {
                    "id": 1,
                    "start": "00:00:03.500",
                    "end": "00:00:07.200",
                    "text": "第二段识别的文本内容"
                }
            ]
        }
    }

.. _speech_to_text_ws_service:

Speech-To-Text-WS 服务
=======================

.. _`custom_properties_speech_to_text_ws`:

Custom Properties of its Service Providers
--------------------------------------------

除了在 :ref:`Metadata of AOG Service
Provider` 中定义的常见属性外, 实时语音识别服务提供商还可以将以下属性放入服务提供商元数据的 ``custom_properties`` 字段中。

.. list-table::
   :header-rows: 1

   * - 自定义属性
     - 值
     - 描述
   * - format
     - string
     - 音频格式，**仅支持"pcm"格式**。AOG服务器只接受PCM格式的音频数据。客户端可以输入WAV或MP3文件，但必须在发送前自动转换为PCM格式。
   * - sample_rate
     - integer
     - 采样率，目前仅支持16000
   * - language
     - string
     - 指定音频内容的语言，如"zh"、"en"等
   * - use_vad
     - boolean
     - 是否使用语音活动检测
   * - return_format
     - string
     - 返回格式，当前仅支持"rst"

WebSocket通信流程
--------------------------------------------

实时语音识别服务基于WebSocket协议，允许用户流式提交语音数据，并实时获取识别结果。WebSocket通信采用"指令-事件"模式，客户端发送指令(Action)，服务端返回事件(Event)。

**WebSocket连接**

.. code-block:: text

    ws://localhost:16688/aog/v0.4/services/speech-to-text-ws

**通信流程**

1. 客户端连接WebSocket服务端
2. 客户端发送 ``run-task`` 指令启动任务
3. 服务端返回 ``task-started`` 事件
4. 客户端发送PCM音频数据(二进制)
5. 服务端返回 ``result-generated`` 事件，包含部分识别结果
6. 客户端发送 ``finish-task`` 指令结束任务
7. 服务端返回 ``task-finished`` 事件

客户端指令格式
--------------------------------------------

**1. run-task 指令**

.. code-block:: json

    {
        "task": "speech-to-text-ws",
        "action": "run-task",
        "model": "NamoLi/whipser-large-v3-ov",
        "parameters": {
            "format": "pcm",
            "sample_rate": 16000,
            "language": "zh",
            "use_vad": true,
            "return_format": "text"
        }
    }

**参数说明**

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - JSON 字段
     - 值
     - 是否必需
     - 描述
   * - task
     - string
     - 必需
     - 任务类型，固定为 ``speech-to-text-ws``
   * - action
     - string
     - 必需
     - 动作类型，固定为 ``run-task``
   * - model
     - string
     - 必需
     - 要使用的语音识别模型名称
   * - parameters
     - object
     - 可选
     - 任务参数对象
   * - parameters.format
     - string
     - 可选
     - 音频格式，**仅支持** ``pcm`` **格式**。服务器只接受PCM音频数据。
   * - parameters.sample_rate
     - integer
     - 可选
     - 采样率，当前仅支持 ``16000``，默认为 ``16000``
   * - parameters.language
     - string
     - 可选
     - 语言代码，如 ``zh``、``en``，默认为 ``zh``
   * - parameters.use_vad
     - boolean
     - 可选
     - 是否使用语音活动检测，默认为 ``true``
   * - parameters.return_format
     - string
     - 可选
     - 返回格式，默认为 ``text``

**2. finish-task 指令**

.. code-block:: json

    {
        "task": "speech-to-text-ws",
        "action": "finish-task",
        "task_id": "2bf83b9a-baeb-4fda-8d9a-xxxxxxxxxxxx",
        "model": "NamoLi/whipser-large-v3-ov"
    }

**参数说明**

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - JSON 字段
     - 值
     - 是否必需
     - 描述
   * - task
     - string
     - 必需
     - 任务类型，固定为 ``speech-to-text-ws``
   * - action
     - string
     - 必需
     - 动作类型，固定为 ``finish-task``
   * - task_id
     - string
     - 必需
     - 服务端返回的任务ID
   * - model
     - string
     - 必需
     - 使用的模型名称，与 ``run-task`` 中的一致

服务端事件格式
--------------------------------------------

**1. task-started 事件**

.. code-block:: json

    {
        "header": {
            "task_id": "2bf83b9a-baeb-4fda-8d9a-xxxxxxxxxxxx",
            "event": "task-started"
        },
        "payload": {}
    }

**参数说明**

.. list-table::
   :header-rows: 1
   :widths: 20 30 50

   * - JSON 字段
     - 值
     - 描述
   * - header.task_id
     - string
     - 服务端生成的任务ID，后续所有通信都需要携带此ID
   * - header.event
     - string
     - 事件类型，固定为 ``task-started``
   * - payload
     - object
     - 空对象，预留扩展

**2. result-generated 事件**

.. code-block:: json

    {
        "header": {
            "task_id": "2bf83b9a-baeb-4fda-8d9a-xxxxxxxxxxxx",
            "event": "result-generated"
        },
        "payload": {
            "output": {
                "sentence": {
                    "begin_time": 170,
                    "end_time": null,
                    "text": "好，我们的一个"
                }
            }
        }
    }

**参数说明**

.. list-table::
   :header-rows: 1
   :widths: 20 30 50

   * - JSON 字段
     - 值
     - 描述
   * - header.task_id
     - string
     - 任务ID
   * - header.event
     - string
     - 事件类型，固定为 ``result-generated``
   * - payload.output.sentence.begin_time
     - integer
     - 开始时间(毫秒)
   * - payload.output.sentence.end_time
     - integer/null
     - 结束时间(毫秒)，可能为null
   * - payload.output.sentence.text
     - string
     - 识别的文本内容

**3. task-finished 事件**

.. code-block:: json

    {
        "header": {
            "task_id": "2bf83b9a-baeb-4fda-8d9a-xxxxxxxxxxxx",
            "event": "task-finished"
        },
        "payload": {}
    }

**参数说明**

.. list-table::
   :header-rows: 1
   :widths: 20 30 50

   * - JSON 字段
     - 值
     - 描述
   * - header.task_id
     - string
     - 任务ID
   * - header.event
     - string
     - 事件类型，固定为 ``task-finished``
   * - payload
     - object
     - 空对象，预留扩展

**4. task-failed 事件**

.. code-block:: json

    {
        "header": {
            "task_id": "2bf83b9a-baeb-4fda-8d9a-xxxxxxxxxxxx",
            "event": "task-failed",
            "error_code": "CLIENT_ERROR",
            "error_message": "request timeout after 23 seconds."
        },
        "payload": {}
    }

**错误代码说明**

.. list-table::
   :header-rows: 1
   :widths: 20 80

   * - 错误代码
     - 描述
   * - CLIENT_ERROR
     - 客户端错误，如无效的请求参数、任务ID等
   * - SERVER_ERROR
     - 服务器内部错误
   * - MODEL_ERROR
     - 模型处理错误

示例
--------------

**服务配置示例**

.. code-block:: json

    {
        "provider_name": "local_openvino_speech",
        "service_name": "speech",
        "service_source": "local",
        "desc": "Local OpenVINO speech recognition service",
        "api_flavor": "openvino",
        "method": "POST",
        "url": "http://localhost:9000",
        "auth_type": "none",
        "auth_key": "",
        "models": [
            "NamoLi/whipser-large-v3-ov"
        ]
    }