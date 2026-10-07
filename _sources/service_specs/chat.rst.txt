===================================
AOG Chat 服务相关
===================================

Chat 服务
=====================

.. _`custom_properties_chat`:

Custom Properties of its Service Providers
--------------------------------------------

除了在 :ref:`Metadata of AOG Service
Provider` 中定义的常见属性外, 聊天服务提供商还可以将以下属性放入服务提供商元数据的 ``custom_properties`` 字段中。

.. list-table::
   :header-rows: 1

   * - 自定义属性
     - 值
     - 描述
   * - max_input_tokens
     - integer
     - 上下文窗口宽度或允许输入的最大token数

请求格式
--------------------------------------------

.. _`header_chat`:

请求头
___________

参见 :ref:`Common Fields in Header of Request`


.. _`request_chat`:

请求
______________

除了在 :ref:`Common Fields in Request Body` 中定义的字段外，，服务在其请求 JSON 体中也可能包含以下字段：

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 附加 JSON 字段
     - 值
     - 是否必需
     - 描述
   * - messages
     - 参见 :ref:`message_type`
     - 必填
     - 聊天消息，可能包含对话历史
   * - seed
     - integer
     - 可选
     - 有助于返回确定性结果
   * - temperature
     - number between 0 to 2, and default is 1
     - 可选
     - 提高温度将使模型回答更具创造性。
   * - top_p
     - float
     - 可选
     - 更高的 top_p 值导致文本更加多样化，而较低的值（例如，0.5）则产生更加专注和保守的文本。默认值为 0.9。

.. _`response_chat`:

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
   * - message
     - 参见 :ref:`message_type`
     - 必填
     - returned message
   * - finished
     - ``true`` or ``false``
     - 必填
     - 对于流模式中的最后一条消息是 ``true`` ， 否则为 ``false`` 。
       对于同步模式始终为 ``true``。
   * - finish_reason
     - stop, length, function_call, or null
     - finished = true 时必填
     - | stop 停止正常结束
       | length 达到最大长度
       | function_call 函数调用结束
       | null 尚未完成，完成状态应为 false


示例
--------------

发送请求

.. code-block:: shell

    curl https://localhost:16688/aog/v0.2/services/chat\
    -H "Content-Type: application/json" \
    -d '{
        "model": "deepseek-r1:7b",
        "stream": true,
        "messages": [
            {
                "role": "user",
                "content": "你好！"
            }
        ]
    }'

返回响应

.. code-block:: json

    {
        "created_at": "2025-03-11T06:38:36.1349763Z",
        "finish_reason": "stop",
        "finished": true,
        "id": "49487566988534527779",
        "message": {
            "content": "<think>\n\n</think>\n\n您好！我是由中国的深度求索（DeepSeek）公司开发的智能助手DeepSeek-R1。如您有任何任何问题，我会尽我所能为您提供帮助。",
            "role": "assistant"
        },
        "model": "deepseek-r1:7b"
    }


Generate 服务
=====================

.. _`custom_properties_generate`:

Custom Properties of its Service Providers
--------------------------------------------

除了在 :ref:`Metadata of AOG Service Provider` 中定义的常见属性外，生成服务提供商还可以将以下属性放入服务提供商元数据的 ``custom_properties`` 字段中。

.. list-table::
   :header-rows: 1

   * - 自定义属性
     - 值
     - 描述
   * - max_input_tokens
     - integer
     - 上下文窗口宽度或允许输入的最大token数

请求格式
--------------------------------------------

.. _`header_generate`:

请求头
___________

参见 :ref:`Common Fields in Header of Request`

.. _`request_generate`:

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
   * - prompt
     - string
     - 必填
     - 需要生成内容的提示词
   * - stream
     - boolean
     - 可选
     - 是否流式返回
   * - think
     - boolean
     - 可选
     - 是否返回思考内容
   * - images
     - Array
     - 可选
     - 图片的 base64 编码

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
   * - success
     - boolean
     - 必填
     - 请求是否成功
   * - message
     - object
     - 必填
     - 生成的内容及相关信息
   * - model
     - string
     - 必填
     - 使用的生成模型
   * - created_at
     - string
     - 必填
     - 响应生成时间
   * - finish_reason
     - string
     - 必填
     - 结束原因
   * - aog_info
     - object
     - 可选
     - 服务端信息

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - message 字段
     - 类型
     - 是否必需
     - 描述
   * - created_at
     - string
     - 必填
     - 消息生成时间
   * - finish_reason
     - string
     - 必填
     - 结束原因
   * - finished
     - boolean
     - 必填
     - 是否完成
   * - id
     - string
     - 必填
     - 消息ID
   * - model
     - string
     - 必填
     - 使用的模型
   * - response
     - string
     - 必填
     - 生成的文本内容

示例
--------------

发送请求

.. code-block:: shell

    curl https://localhost:16688/aog/v0.4/services/generate\
    -H "Content-Type: application/json" \
    -d '{
        "model": "gemma3:4b",
        "stream": false,
        "think": false,
        "prompt": "根据图片内容，生成一首短诗",
        "images": ['iVBORw0KGgoAAAANSUhEUgAAAgAAAAIACAIAA...']
    }'

返回响应

.. code-block:: json

    {
      "success": true,
      "message": {
        "created_at": "2025-08-14T08:12:24.4963364Z",
        "finish_reason": "stop",
        "finished": true,
        "id": "115177740913632811435",
        "model": "gemma3:4b",
        "response": "绿衣孩童笑\n轮盘映夕阳\n静物伴光照"
      },
      "model": "gemma3:4b",
      "created_at": "2025-08-14T08:12:24.4963364Z",
      "finish_reason": "stop",
      "aog_info": {
        "served_by": null,
        "served_by_api_flavor": null,
        "response_time": null
      }
    }


Embed 服务
=====================


自定义服务提供商属性
--------------------------------------------

除了在 :ref:`Metadata of AOG Service
Provider` 中定义的常见属性外，聊天服务提供商还可以将以下属性放入服务提供商元数据的 ``custom_properties`` 字段中。

.. list-table::
   :header-rows: 1

   * - 自定义属性
     - 值
     - 描述
   * - max_input_tokens
     - integer
     - 上下文窗口宽度或允许输入的最大token数

请求格式
--------------------------------------------

请求头
___________

参见 :ref:`Common Fields in Header of Request`

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
   * - input
     - Array of string
     - 必填
     - 输入文本用于嵌入
   * - model
     - string
     - 可选
     - embedding模型


Response Schema
--------------------------------------------

除了在 :ref:`Common Fields in Response Body` 中定义的字段外，该服务在其响应 JSON 体中可能还有以下字段：

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 附加 JSON 字段
     - 值
     - 是否必需
     - 描述
   * - model
     - string
     - 必填
     - embedding模型
   * - id
     - string
     - 必填
     - 请求id
   * - data
     - array of object
     - 必填
     - embedding结果

示例
----------------

返回的嵌入可能如下所示

.. code-block:: json

    {
      "data": [
        {
          "embedding": [
            -0.0695386752486229, 0.030681096017360687
          ],
          "index": 0,
          "object": "embedding"
        },
        {
          "embedding": [
            -0.06348952651023865, 0.060446035116910934
          ],
          "index": 5,
          "object": "embedding"
        }
      ],
      "model": "text-embedding-v3",
      "id": "73591b79-d194-9bca-8bb5-xxxxxxxxxxxx"
    }

