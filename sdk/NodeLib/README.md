# AOGLib使用说明

## 1. 介绍

AOGLib 将协助开发者使用 AOG（白泽模型框架）。

现在 AOGLib 提供了以下功能：

检查 aog 服务是否存在

检查 aog.exe 是否下载

下载 aog.exe

## 2. 使用

首先在 NodeJS 项目中安装该 Node Module：


``` sh
npm install aog-lib-1.0.0.tgz
```

然后在项目中引入该 Node Module：

``` JavaScript
const AOGLib = require('aog-lib');

const aog = new AOGLib();

// 检查 aog 服务是否存在
aog.isAOGAvailable().then((result) => {
    console.log(result);
});

// 检查 aog.exe 是否下载
const existed = aog.isAOGExisted();
console.log(existed);

// 下载 aog.exe
aog.downloadAOG().then((result) => {
    console.log(result);
});

// 启动 aog 服务
aog.startAOG().then((result) => {
    console.log(result);
});

// 查看当前服务
aog.getServices().then((result) => {
    console.log(result);
});

// 创建新服务
const data = {
    service_name: "chat/embed/generate/text-to-image",
    service_source: "remote/local",
    hybrid_policy: "default/always_local/always_remote",
    flavor_name: "ollama/openai/...",
    provider_name: "local_ollama_chat/remote_openai_chat/...",
    auth_type: "none/apikey",
    auth_key: "your_api_key",
}; // 必填service_name, service_source, hybrid_policy, flavor_name, provider_name

aog.installService(data).then((result) => {
    console.log(result);
});

// 更新服务
const data = {
    service_name: "chat/embed/generate/text-to-image",
    hybrid_policy: "default/always_local/always_remote",
    remote_provider: "",
    local_provider: ""
}; // 必填service_name

aog.updateService(data).then((result) => {
    console.log(result);
});

// 查看模型
aog.getModels().then((result) => {
    console.log(result);
});

// 安装模型
const data = {
    model_name: "llama2",
    service_name: "chat/embed/generate/text-to-image",
    service_source: "remote/local",
    provider_name: "local_ollama_chat/remote_openai_chat/...",
}; // 必填model_name, service_name, service_source

aog.installModel(data).then((result) => {
    console.log(result);
});

// 卸载模型
const data = {
    model_name: "llama2",
    service_name: "chat/embed/generate/text-to-image",
    service_source: "remote/local",
    provider_name: "local_ollama_chat/remote_openai_chat/...",
}; // 必填model_name, service_name, service_source

aog.deleteModel(data).then((result) => {
    console.log(result);
});

// 查看服务提供商
aog.getServiceProviders().then((result) => {
    console.log(result);
});

// 新增模型提供商
const data = {
    service_name: "chat/embed/generate/text-to-image",
    service_source: "remote/local",
    flavor_name: "ollama/openai/...",
    provider_name: "local_ollama_chat/remote_openai_chat/...",
    desc: "",
    method: "",
    auth_type: "none/apikey",
    auth_key: "your_api_key",
    models: ["qwen2:7b", "deepseek-r1:7b", ...],
    extra_headers: {},
    extra_json_body: {},
    properties: {}
}; // 必填service_name, service_source, flavor_name, provider_name
aog.installServiceProvider(data).then((result) => {
    console.log(result);
});

// 更新模型提供商
const data = {
    service_name: "chat/embed/generate/text-to-image",
    service_source: "remote/local",
    flavor_name: "ollama/openai/...",
    provider_name: "local_ollama_chat/remote_openai_chat/...",
    desc: "",
    method: "",
    auth_type: "none/apikey",
    auth_key: "your_api_key",
    models: ["qwen2:7b", "deepseek-r1:7b", ...],
    extra_headers: {},
    extra_json_body: {},
    properties: {}
}; // 必填service_name, service_source, flavor_name, provider_name

aog.updateServiceProvider(data).then((result) => {
    console.log(result);
});

// 删除服务提供商
const data = {
    provider_name: ""
};

aog.deleteServiceProvider(data).then((result) => {
    console.log(result);
});

// 导入配置文件
aog.importConfig("path/to/.aog").then((result) => {
    console.log(result);
});

// 导出配置文件
const data = {
    service_name: "chat/embed/generate/text-to-image"
};

aog.exportConfig(data).then((result) => { // 不填data则导出全部
    console.log(result);
});

// 获取模型列表（查看ollama的模型）
// aog.getModelsAvailiable() 方法已移除或重命名，请使用 getModels()
aog.getModels().then((result) => {
    console.log(result);
});

// 获取推荐模型列表
aog.getModelsRecommended().then((result) => {
    console.log(result);
});

// 获取支持模型列表
const data = {
    service_source: "remote/local",
    flavor: "ollama/openai/..." // local 则默认为ollama
}; // 必填service_source, flavor
aog.getModelsSupported(data).then((result) => {
    console.log(result);
});

// Chat服务（流式）
const data = {
    model: "deepseek-r1:7b",
    stream: true,
    messages: [
        {
            role: "user",
            content: "你好"
        }
    ],
    temperature: 0.7,
    max_tokens: 100,
}

aog.chat(data).then((chatStream) => {
    chatStream.on('data', (data) => {
        console.log(data);
    });
    chatStream.on('error', (error) => {
        console.error(error);
    });
    chatStream.on('end', () => {
        console.log('Chat stream ended');
    });
});

// Chat服务（非流式）
const data = {
    model: "deepseek-r1:7b",
    stream: false,
    messages: [
        {
            role: "user",
            content: "你好"
        }
    ],
    temperature: 0.7,
    max_tokens: 100,
}

aog.chat(data).then((result) => {
    console.log(result);
});

// 生文服务（流式）
const data = {
    model: "deepseek-r1:7b",
    stream: true,
    prompt: "你好",
}
aog.generate(data).then((generateStream) => {
    generateStream.on('data', (data) => {
        console.log(data);
    });
    generateStream.on('error', (error) => {
        console.error(error);
    });
    generateStream.on('end', () => {
        console.log('Generate stream ended');
    });
});

// 生文服务（非流式）
const data = {
    model: "deepseek-r1:7b",
    stream: false,
    prompt: "你好",
}
aog.generate(data).then((result) => {
    console.log(result);
});

// 文生图服务
const data = {
    model: "wanx2.1-t2i-turbo",
    prompt: "一间有着精致窗户的花店，漂亮的木质门，摆放着花朵",
}

aog.textToImage(data).then((result) => {
    console.log(result);
});

// 语音识别服务
const data = {
    model: "NamoLi/whisper-large-v3-ov",
    audio: "PATH/TO/YOUR/AUDIO/FILE.wav",
    language: "zh"
}

aog.speechToText(data).then(response => {
    console.log( response);
});

// 实时语音识别服务
const speechStream = oadin.SpeechToTextStream({
  model: "NamoLi/whisper-large-v3-ov",
  language: "zh",
  sampleRate: 16000
});

speechStream.on('open', () => {
  console.log('✅ WebSocket 连接已建立');
});

speechStream.on('taskStarted', ({ taskId }) => {
  console.log(`🚀 任务已启动, ID: ${taskId}`);
});

speechStream.on('finished', ({ text, taskId }) => {
  console.log(`🏁 任务完成 (ID: ${taskId}):`, text);
});

speechStream.on('error', (err) => {
  console.error('❌ 错误:', err.message);
});

speechStream.on('close', () => {
  console.log('🔌 连接已关闭');
});

const audioPath = "PATH/TO/YOUR/AUDIO/FILE.MP3";
const CHUNK_SIZE = 32000; // 合适的块大小

if (!fs.existsSync(audioPath)) {
  console.error(`❌ 文件不存在: ${audioPath}`);
  process.exit(1);
}

const readStream = fs.createReadStream(audioPath, { highWaterMark: CHUNK_SIZE });

speechStream.on('taskStarted', () => {
  console.log('📤 开始发送音频数据...');
  
  let sending = 0;
  let ended = false;

  readStream.on('data', (chunk) => {
    sending++;
    const canWrite = speechStream.write(chunk);
    sending--;
    if (!canWrite) {
      readStream.pause();
      speechStream.once('drain', () => {
        readStream.resume();
      });
    }
    if (ended && sending === 0) {
      speechStream.end();
    }
  });

  readStream.on('end', () => {
    ended = true;
    if (sending === 0) {
      console.log('📭 音频发送完毕');
      speechStream.end();
    }
  });
});

speechStream.on('error', () => {
  readStream.destroy();
});
```
