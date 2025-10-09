===================================
AOG RAG 服务相关
===================================

File Upload
=====================
请求格式
--------------------------------------------

.. _`header_chat`:

请求头
___________

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 字段名
     - 值
     - 是否必需
     - 描述
   * - content-type
     - multipart/form-data
     - 必填
     - 请求数据格式


.. _`request_chat`:

请求
______________

响应格式
--------------------------------------------

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 字段
     - 值
     - 是否必需
     - 描述
   * - business_code
     - int
     - 必填
     - 业务代码
   * - message
     - string
     - 必填
     - 响应文本信息
   * - data
     - Object
     - 必填
     - 响应数据
   * - data.file_id
     - string
     - 必填
     - 文件唯一id
示例
--------------

发送请求

.. code-block:: shell

    curl -X POST https://localhost:16688/aog/v0.2/rag/file \
      -H "Content-Type: multipart/form-data" \
      -F "file=@/path/to/your/file.txt"

返回响应

.. code-block:: json

    {
         "business_code": 50000,
         "message": ""，
         "data": {
             "file_id": "asdgasbdohaklndlak123"
         }
    }


Get File
=====================
请求格式
--------------------------------------------

.. _`header_generate`:

请求头
___________

参见 :ref:`Common Fields in Header of Request`

.. _`request_generate`:

请求
______________

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 字段名
     - 值
     - 是否必需
     - 描述
   * - file_id
     - string
     - 必填
     - 文件id

响应格式
--------------------------------------------

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 字段
     - 值
     - 是否必需
     - 描述
   * - business_code
       - int
       - 必填
       - 业务代码
   * - message
       - string
       - 必填
       - 响应文本信息
   * - data
       - Object
       - 必填
       - 响应数据
   * - data.id
       - int
       - 必填
       - 文件数据库id
   * - data.file_id
       - string
       - 必填
       - 文件id
   * - data.file_name
       - string
       - 必填
       - 文件名
   * - data.file_type
       - string
       - 必填
       - 文件类型
   * - data.file_path
       - string
       - 必填
       - 文件路径
   * - data.status
       - int
       - 必填
       - 文件状态，1-processing, 2-done, 3-failed
   * - data.embed_model
       - string
       - 必填
       - 向量化模型，默认 bge-m3:567m
   * - data.created_at
       - string
       - 必填
       - 数据创建时间
   * - data.updated_at
       - string
       - 必填
       - 数据更新时间

示例
--------------

发送请求

.. code-block:: shell

    curl https://localhost:16688/aog/v0.2/rag/file\
    -H "Content-Type: application/json" \
    -d '{
        "file_ids": "asdgasbdohaklndlak123",
    }'

返回响应

.. code-block:: json

    {
      "business_code": 50000,
      "message": ""，
      "data": {
         id:1
         file_id: "1231231hdajkdhnask",
         file_name: "123123",
         file_type: "txt",
         file_path: "/path/123123.txt",
         status: 1 # 1-processing | 2-done | 3-failed
         embed_model: "bge-m3:567m",
         created_at: "2025-09-05 00:00:00",
         updated_at: "2025-09-05 00:00:00"
      },
    }


Get Files
=====================
请求格式
--------------------------------------------

.. _`header_generate`:

请求头
___________

参见 :ref:`Common Fields in Header of Request`

.. _`request_generate`:

请求
______________

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 字段名
     - 值
     - 是否必需
     - 描述

响应格式
--------------------------------------------

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 字段
     - 值
     - 是否必需
     - 描述
   * - business_code
       - int
       - 必填
       - 业务代码
   * - message
       - string
       - 必填
       - 响应文本信息
   * - data
       - Array
       - 必填
       - 响应数据
   * - data[].id
       - int
       - 必填
       - 文件数据库id
   * - data[].file_id
       - string
       - 必填
       - 文件id
   * - data[].file_name
       - string
       - 必填
       - 文件名
   * - data[].file_type
       - string
       - 必填
       - 文件类型
   * - data[].file_path
       - string
       - 必填
       - 文件路径
   * - data[].status
       - int
       - 必填
       - 文件状态，1-processing, 2-done, 3-failed
   * - data[].embed_model
       - string
       - 必填
       - 向量化模型，默认 bge-m3:567m
   * - data[].created_at
       - string
       - 必填
       - 数据创建时间
   * - data[].updated_at
       - string
       - 必填
       - 数据更新时间

示例
--------------

发送请求

.. code-block:: shell

    curl https://localhost:16688/aog/v0.2/rag/files\
    -H "Content-Type: application/json" \

返回响应

.. code-block:: json

    {
      "business_code": 50000,
      "message": ""，
      "data": [
          {
             id:1
             file_id: "1231231hdajkdhnask",
             file_name: "123123",
             file_type: "txt",
             file_path: "/path/123123.txt",
             status: 1 # 1-processing | 2-done | 3-failed
             embed_model: "bge-m3:567m",
             created_at: "2025-09-05 00:00:00",
             updated_at: "2025-09-05 00:00:00"
          },
      ]
    }


Delete File
=====================
请求格式
--------------------------------------------

.. _`header_generate`:

请求头
___________

参见 :ref:`Common Fields in Header of Request`

.. _`request_generate`:

请求
______________

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 字段名
     - 值
     - 是否必需
     - 描述
   * - file_id
     - string
     - 必填
     - 文件id

响应格式
--------------------------------------------

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 字段
     - 值
     - 是否必需
     - 描述
   * - business_code
       - int
       - 必填
       - 业务代码
   * - message
       - string
       - 必填
       - 响应文本信息
   * - data
       - Object
       - 必填
       - 响应数据

示例
--------------

发送请求

.. code-block:: shell

    curl -X DELETE https://localhost:16688/aog/v0.2/rag/file\
    -H "Content-Type: application/json" \

返回响应

.. code-block:: json

    {
      "business_code": 50000,
      "message": ""，
      "data": {}
    }


Retrieval 服务
=====================
请求格式
--------------------------------------------

.. _`header_generate`:

请求头
___________

参见 :ref:`Common Fields in Header of Request`

.. _`request_generate`:

请求
______________

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 字段名
     - 值
     - 是否必需
     - 描述
   * - file_ids
     - Array of string
     - 必填
     - 使用哪些文件进行检索
   * - model
     - string
     - 可选
     - 大语音模型
   * - text
     - string
     - 必填
     - 需要检索的文本

响应格式
--------------------------------------------

除了在 :ref:`Common Fields in Response Body` 中定义的字段外，该服务在其响应 JSON 体中可能还有以下字段：

.. list-table::
   :header-rows: 1
   :widths: 10 35 10 45

   * - 字段
     - 值
     - 是否必需
     - 描述
   * - business_code
       - int
       - 必填
       - 业务代码
   * - message
       - string
       - 必填
       - 响应文本信息
   * - data
       - Object
       - 必填
       - 响应数据
   * - data.model
       - string
       - 必填
       - 大语言模型
   * - data.content
       - string
       - 必填
       - 检索结果

示例
--------------

发送请求

.. code-block:: shell

    curl -X POST https://localhost:16688/aog/v0.2/rag/retrieval\
    -H "Content-Type: application/json" \
    -d '{
        "model": "qwen3:7b",
        "file_ids": ["asdgasbdohaklndlak123", "....."],
        "text": "......."
    }'

返回响应

.. code-block:: json

    {
      "business_code": 50000,
      "message": ""，
      "model": "qwen3:7b",
      "content": ".........."
    }

