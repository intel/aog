===================================
AOG Text-to-image 服务相关
===================================

Text-to-image 服务
=====================

.. _`custom_properties_chat`:

Custom Properties of its Service Providers
--------------------------------------------

除了在 :ref:`Metadata of AOG Service
Provider` 中定义的常见属性外, 文生图服务提供商还可以将以下属性放入服务提供商元数据的 ``custom_properties`` 字段中。

.. list-table::
   :header-rows: 1

   * - 自定义属性
     - 值
     - 描述
   * - prompt
     - string
     - 用来描述生成图像中期望包含的元素和视觉特点


请求格式
--------------------------------------------

.. _`header_text-to-image`:

请求头
___________

参见 :ref:`Common Fields in Header of Request`


.. _`request_text-to-image`:

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
   * - seed
     - integer
     - 可选
     - 有助于返回确定性结果
   * - n
     - integer
     - 可选
     - 单次prompt生成的图片数量，默认值为1，最大值为4
   * - size
     - string
     - 可选
     - 生成图片的尺寸(长*宽)，示例：
       - 512x512 （默认）
       - 1024x1024
       - 2048x2048

.. _`response_text-to-image`:

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
   * - url
     - ``local_path`` or ``url``
     - 必填
     - 生成的图片的本地路径或URL，基于云端服务会输出 ``Url``，本地服务实际输出为本机图片路径



示例
--------------

发送请求

.. code-block:: shell

    curl https://localhost:16688/aog/v0.3/services/text-to-image\
    -H "Content-Type: application/json" \
    -d '{
            "model": "wanx2.1-t2i-turbo",
            "prompt": "一间有着精致窗户的花店，漂亮的木质门，摆放着花朵",
            "n": 1,
            "size": "1024x1024",
        }'

返回响应一

.. code-block:: json

    {
        "data": {
            "url": [
                "https://dashscope-result-wlcb-acdr-1.oss-cn-wulanchabu-acdr-1.aliyuncs.com/1d/4e/20250319/b0fe3396/018c4baa-9f42-4946-8750-14a9fa74e1af885741332.png?Expires=1742442524&OSSAccessKeyId=<Your Access Key>&Signature=<Your Signature>"
            ]
        },
        "id": "ab967cd8-392f-90d9-a2b2-92bf1792cd7f"
    }

返回响应二
.. code-block:: json

    {
        "data": {
            "url": [
                "/Users/xxxx/Downloads/2025051516065812420.png",
                "/Users/xxxx/Downloads/2025051516065846881.png"
            ]
        }
    }




