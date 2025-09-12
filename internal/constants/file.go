package constants

const (
	FileAudioWav       = "wav"
	FileAudioMp3       = "mp3"
	FileAudioM4a       = "m4a"
	FileAudioOgg       = "ogg"
	FileAudioFlac      = "flac"
	FileAudioAac       = "aac"
	FileAudioMp4       = "mp4"
	FileImagePng       = "png"
	FileImageJpg       = "jpg"
	FileDataTypeUrl    = "url"
	FileDataTypePath   = "path"
	FileDataTypeBase64 = "base64"
)

var (
	SupportFileDataType = []string{FileDataTypeUrl, FileDataTypePath, FileDataTypeBase64}
	SupportAudioType    = []string{FileAudioWav, FileAudioMp3}
)
