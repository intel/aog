===================================
AOG Image相关服务
===================================

Text-to-image 服务
=====================

.. _`custom_properties_text_to_image`:

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
       - 512*512 （默认）
       - 1024*1024
       - 2048*2048
   * - 附加 JSON 字段
     - 值
     - 是否必需
     - 描述
   * - image
     - string
     - 可选
     - 参考图片的地址/路径/base64
   * - image_type
     - ``path``, ``url`` or ``base64``
     - 可选
     - image 字段是什么类型

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
     - 可选
     - 生成的图片的本地路径或URL，基于云端服务会输出 ``Url``，本地服务实际输出为本机图片路径

示例
--------------

发送请求一

.. code-block:: shell

    curl https://localhost:16688/aog/v0.2/services/text-to-image\
    -H "Content-Type: application/json" \
    -d '{
            "model": "wanx2.1-t2i-turbo",
            "prompt": "一间有着精致窗户的花店，漂亮的木质门，摆放着花朵",
            "n": 1,
            "size": "1024*1024",
        }'

发送请求二

.. code-block:: shell

    curl https://localhost:16688/aog/v0.2/services/text-to-image\
    -H "Content-Type: application/json" \
    -d '{
            "model": "wanx2.1-t2i-turbo",
            "prompt": "增加更多花朵",
            "n": 2,
            "size": "1024*1024",
            "image": "path/to/image",
            "image_type": "path"
        }'

返回响应一

.. code-block:: json

    {
        "business_code": 200,
        "message": "success",
        "data": {
            "id": "xxxxx",
            "url": [
                "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/1d/22/xxxx/xxxxx/xxxxxx-xxxxxx-xxxx-xxxx-1-1.png?Expires=xxxx27&OSSAccessKeyId=xxxx&Signature=xxxx%2BecZxxxx70%3D"
            ]
        }
    }

返回响应二

.. code-block:: json

    {
        "business_code": 200,
        "message": "success",
        "data": {
            "id": "xxxxx",
            "url": [
                "/Users/xxxx/Downloads/2025051516065812420.png",
                "/Users/xxxx/Downloads/2025051516065846881.png"
            ]
        }
    }


Image-to-image 服务
=====================

.. _`custom_properties_image_to_image`:

Custom Properties of its Service Providers
--------------------------------------------

除了在 :ref:`Metadata of AOG Service Provider` 中定义的常见属性外, 图生图服务提供商还可以将以下属性放入服务提供商元数据的 ``custom_properties`` 字段中。

.. list-table::
   :header-rows: 1

   * - 自定义属性
     - 值
     - 描述
   * - prompt
     - string
     - 用来描述生成图像的风格或变化
   * - image
     - string
     - 输入的原始图片（路径、url或base64）

请求格式
--------------------------------------------

.. _`header_image-to-image`:

请求头
___________

参见 :ref:`Common Fields in Header of Request`

.. _`request_image-to-image`:

目前只支持了远程的图生图服务。

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
   * - images
     - string
     - 必需
     - 输入的原始图片
   * - prompt
     - string
     - 可选
     - 用于指导生成图片的文本
   * - image
     - string
     - 必需
     - 生成图片的地址/路径/base64
   * - image_type
     - ``path``, ``url`` or ``base64``
     - 必需
     - image 字段是什么类型


示例
--------------

发送请求

.. code-block:: shell

    curl https://localhost:16688/aog/v0.2/services/image-to-image\
    -H "Content-Type: application/json" \
    -d '{
            "model": "wanx2.1-i2i-turbo",
            "image": ,
            "prompt": "改为油画风格",
            "n": 1
        }'

返回响应

.. code-block:: json

    {
      "business_code": 200,
      "message": "success",
      "data": {
        "id": "ec9b1fb2-b6ff-9f71-bf27-f250a7bcbe66",
        "url": [
          "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/1d/22/xxxx/xxxxx/xxxxxx-xxxxxx-xxxx-xxxx-1-1.png?Expires=xxxx27&OSSAccessKeyId=xxxx&Signature=xxxx%2BecZxxxx70%3D"
        ]
      }
    }


Image-to-video 服务
=====================

.. _`custom_properties_image_to_video`:

Custom Properties of its Service Providers
--------------------------------------------

除了在 :ref:`Metadata of AOG Service Provider` 中定义的常见属性外, 图生视频服务提供商还可以将以下属性放入服务提供商元数据的 ``custom_properties`` 字段中。

.. list-table::
   :header-rows: 1

   * - 自定义属性
     - 值
     - 描述
   * - image
     - string
     - 输入的原始图片（路径、url或base64）
   * - prompt
     - string
     - 可选，生成视频的风格或描述

请求格式
--------------------------------------------

.. _`header_image-to-video`:

请求头
___________

参见 :ref:`Common Fields in Header of Request`

.. _`request_image-to-video`:

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
   * - model
     - string
     - 必需
     - 使用的模型名称
   * - prompt
     - string
     - 可选
     - 用于指导生成图片的文本
   * - image
     - string
     - 必需
     - 生成图片的地址/路径/base64
   * - image_type
     - ``path``, ``url`` or ``base64``
     - 必需
     - image 字段是什么类型

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
   * - id
     - string
     - 必需
     - 请求ID
   * - data
     - string
     - 必需
     - 生成视频的URL

示例
--------------

发送请求

.. code-block:: shell

    curl https://localhost:16688/aog/v0.2/services/image-to-video\
    -H "Content-Type: application/json" \
    -d '{
            "model": "wan2.2-i2v-plus",
            "image": "https://example.com/input.png",
            "image_type": "url"
            "prompt": "让猫在草地上奔跑",
        }'

返回响应

.. code-block:: json

    {
        "id": "ec9b1fb2-b6ff-9f71-bf27-f250a7bcbe66",
        "data": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/xxx/xxx.mp4"
    }
