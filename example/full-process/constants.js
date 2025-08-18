// 当前项目目录
const serverImagePath = '/uploads/images';
const serverImageFullPath = __dirname + '/uploads/images';

const defaultChatModel = "qwen2.5:0.5b";
const defaultGenerateModel = "gemma3:4b";

const defaultTextToSpeechModel = "qwen-tts";
const defaultTextToSpeechVoice = "Cherry";

const defaultSpeechToTextModel = "NamoLi/whisper-large-v3-ov";

const defaultTextToImageModel = "OpenVINO/LCM_Dreamshaper_v7-fp16-ov";
const defaultImageToImageModel = "wanx2.1-imageedit";
const defaultLocalSize = "512*512";
const defaultRemoteSize = "1024*1024";


const generatePrompt = "根据图片内容，生成一首短诗，格式不限，注意分行，只需要返回诗的内容，不要返回其他语句。";


module.exports = {
    serverImagePath,
    serverImageFullPath,
    defaultChatModel,
    defaultGenerateModel,
    generatePrompt,
    defaultTextToSpeechModel,
    defaultTextToSpeechVoice,
    defaultSpeechToTextModel,
    defaultTextToImageModel,
    defaultImageToImageModel,
    defaultLocalSize,
    defaultRemoteSize
}