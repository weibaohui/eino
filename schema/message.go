/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package schema

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
	"text/template"

	"github.com/nikolalohinski/gonja"
	"github.com/nikolalohinski/gonja/config"
	"github.com/nikolalohinski/gonja/nodes"
	"github.com/nikolalohinski/gonja/parser"
	"github.com/slongfield/pyfmt"

	"github.com/cloudwego/eino/internal"
	"github.com/cloudwego/eino/internal/generic"
)

func init() {
	internal.RegisterStreamChunkConcatFunc(ConcatMessages)
	internal.RegisterStreamChunkConcatFunc(ConcatMessageArray)
}

// ConcatMessageArray merges aligned slices of messages into a single slice,
// concatenating messages at the same index across the input arrays.
//
// ConcatMessageArray 将对齐的消息切片合并为单个切片，
// 连接输入数组中相同索引处的消息。
func ConcatMessageArray(mas [][]*Message) ([]*Message, error) {
	arrayLen := len(mas[0])

	ret := make([]*Message, arrayLen)
	slicesToConcat := make([][]*Message, arrayLen)

	for _, ma := range mas {
		if len(ma) != arrayLen {
			return nil, fmt.Errorf("unexpected array length. "+
				"Got %d, expected %d", len(ma), arrayLen)
		}

		for i := 0; i < arrayLen; i++ {
			m := ma[i]
			if m != nil {
				slicesToConcat[i] = append(slicesToConcat[i], m)
			}
		}
	}

	for i, slice := range slicesToConcat {
		if len(slice) == 0 {
			ret[i] = nil
		} else if len(slice) == 1 {
			ret[i] = slice[0]
		} else {
			cm, err := ConcatMessages(slice)
			if err != nil {
				return nil, err
			}

			ret[i] = cm
		}
	}

	return ret, nil
}

// FormatType used by MessageTemplate.Format
//
// FormatType 由 MessageTemplate.Format 使用。
type FormatType uint8

const (
	// FString Supported by pyfmt(github.com/slongfield/pyfmt), which is an implementation of https://peps.python.org/pep-3101/.
	//
	// FString 由 pyfmt(github.com/slongfield/pyfmt) 支持，它是 https://peps.python.org/pep-3101/ 的实现。
	FString FormatType = 0
	// GoTemplate https://pkg.go.dev/text/template.
	//
	// GoTemplate https://pkg.go.dev/text/template。
	GoTemplate FormatType = 1
	// Jinja2 Supported by gonja(github.com/nikolalohinski/gonja), which is a implementation of https://jinja.palletsprojects.com/en/3.1.x/templates/.
	//
	// Jinja2 由 gonja(github.com/nikolalohinski/gonja) 支持，它是 https://jinja.palletsprojects.com/en/3.1.x/templates/ 的实现。
	Jinja2 FormatType = 2
)

// RoleType is the type of the role of a message.
//
// RoleType 是消息角色的类型。
type RoleType string

const (
	// Assistant is the role of an assistant, means the message is returned by ChatModel.
	//
	// Assistant 是助手的角色，表示消息由 ChatModel 返回。
	Assistant RoleType = "assistant"
	// User is the role of a user, means the message is a user message.
	//
	// User 是用户的角色，表示消息是用户消息。
	User RoleType = "user"
	// System is the role of a system, means the message is a system message.
	//
	// System 是系统的角色，表示消息是系统消息。
	System RoleType = "system"
	// Tool is the role of a tool, means the message is a tool call output.
	//
	// Tool 是工具的角色，表示消息是工具调用输出。
	Tool RoleType = "tool"
)

// FunctionCall is the function call in a message.
// It's used in Assistant Message.
//
// FunctionCall 是消息中的函数调用。
// 它用于 Assistant Message。
type FunctionCall struct {
	// Name is the name of the function to call, it can be used to identify the specific function.
	// Name 是要调用的函数名称，可用于标识特定函数。
	Name string `json:"name,omitempty"`
	// Arguments is the arguments to call the function with, in JSON format.
	// Arguments 是调用函数的参数，采用 JSON 格式。
	Arguments string `json:"arguments,omitempty"`
}

// ToolCall is the tool call in a message.
// It's used in Assistant Message when there are tool calls should be made.
//
// ToolCall 是消息中的工具调用。
// 当需要进行工具调用时，它用于 Assistant Message。
type ToolCall struct {
	// Index is used when there are multiple tool calls in a message.
	// In stream mode, it's used to identify the chunk of the tool call for merging.
	//
	// Index 用于消息中有多个工具调用时。
	// 在流模式下，它用于标识工具调用块以进行合并。
	Index *int `json:"index,omitempty"`
	// ID is the id of the tool call, it can be used to identify the specific tool call.
	//
	// ID 是工具调用的 id，可用于标识特定工具调用。
	ID string `json:"id"`
	// Type is the type of the tool call, default is "function".
	//
	// Type 是工具调用的类型，默认为 "function"。
	Type string `json:"type"`
	// Function is the function call to be made.
	//
	// Function 是要进行的函数调用。
	Function FunctionCall `json:"function"`

	// Extra is used to store extra information for the tool call.
	//
	// Extra 用于存储工具调用的额外信息。
	Extra map[string]any `json:"extra,omitempty"`
}

// ImageURLDetail is the detail of the image url.
//
// ImageURLDetail 是图片 url 的细节。
type ImageURLDetail string

const (
	// ImageURLDetailHigh means the high quality image url.
	//
	// ImageURLDetailHigh 表示高质量图片 url。
	ImageURLDetailHigh ImageURLDetail = "high"
	// ImageURLDetailLow means the low quality image url.
	//
	// ImageURLDetailLow 表示低质量图片 url。
	ImageURLDetailLow ImageURLDetail = "low"
	// ImageURLDetailAuto means the auto quality image url.
	//
	// ImageURLDetailAuto 表示自动质量图片 url。
	ImageURLDetailAuto ImageURLDetail = "auto"
)

// MessagePartCommon represents the common abstract components for input and output of multi-modal types.
//
// MessagePartCommon 表示多模态类型输入和输出的通用抽象组件。
type MessagePartCommon struct {
	// URL is primarily used for HTTP or HTTPS access links.
	// For data in the format 'data:[<mediatype>][;base64],<data>' (the 'data' URL Schema of RFC-2397 (https://www.rfc-editor.org/rfc/rfc2397)),
	// it is recommended to use Base64Data and MIMEType fields separately instead.
	//
	// URL 主要用于 HTTP 或 HTTPS 访问链接。
	// 对于格式为 'data:[<mediatype>][;base64],<data>' 的数据（RFC-2397 的 'data' URL Schema），建议分别使用 Base64Data 和 MIMEType 字段。
	URL *string `json:"url,omitempty"`

	// Base64Data represents the binary data in Base64 encoded string format.
	//
	// Base64Data 表示 Base64 编码字符串格式的二进制数据。
	Base64Data *string `json:"base64data,omitempty"`

	// MIMEType is the mime type , eg."image/png",""audio/wav" etc.
	//
	// MIMEType 是 mime 类型，例如 "image/png", "audio/wav" 等。
	MIMEType string `json:"mime_type,omitempty"`

	// Extra is used to store extra information.
	//
	// Extra 用于存储额外信息。
	Extra map[string]any `json:"extra,omitempty"`
}

// MessageInputImage is used to represent an image part in message.
// Choose either URL or Base64Data.
//
// MessageInputImage 用于表示消息中的图像部分。
// 选择 URL 或 Base64Data。
type MessageInputImage struct {
	MessagePartCommon

	// Detail is the quality of the image url.
	//
	// Detail 是图片 url 的质量。
	Detail ImageURLDetail `json:"detail,omitempty"`
}

// MessageInputAudio is used to represent an audio part in message.
// Choose either URL or Base64Data.
//
// MessageInputAudio 用于表示消息中的音频部分。
// 选择 URL 或 Base64Data。
type MessageInputAudio struct {
	MessagePartCommon
}

// MessageInputVideo is used to represent a video part in message.
// Choose either URL or Base64Data.
//
// MessageInputVideo 用于表示消息中的视频部分。
// 选择 URL 或 Base64Data。
type MessageInputVideo struct {
	MessagePartCommon
}

// MessageInputFile is used to represent a file part in message.
// Choose either URL or Base64Data.
//
// MessageInputFile 用于表示消息中的文件部分。
// 选择 URL 或 Base64Data。
type MessageInputFile struct {
	MessagePartCommon

	// Name represents the filename.
	// Optional.
	//
	// Name 表示文件名。
	// 可选。
	Name string `json:"name,omitempty"`
}

// MessageInputPart represents the input part of message.
//
// MessageInputPart 表示消息的输入部分。
type MessageInputPart struct {
	Type ChatMessagePartType `json:"type"`

	Text string `json:"text,omitempty"`

	// Image is the image input of the part, it's used when Type is "image_url".
	//
	// Image 是部分的图像输入，当 Type 为 "image_url" 时使用。
	Image *MessageInputImage `json:"image,omitempty"`

	// Audio  is the audio input of the part, it's used when Type is "audio_url".
	//
	// Audio 是部分的音频输入，当 Type 为 "audio_url" 时使用。
	Audio *MessageInputAudio `json:"audio,omitempty"`

	// Video is the video input of the part, it's used when Type is "video_url".
	//
	// Video 是部分的视频输入，当 Type 为 "video_url" 时使用。
	Video *MessageInputVideo `json:"video,omitempty"`

	// File is the file input of the part, it's used when Type is "file_url".
	//
	// File 是部分的文件输入，当 Type 为 "file_url" 时使用。
	File *MessageInputFile `json:"file,omitempty"`

	// Extra is used to store extra information.
	//
	// Extra 用于存储额外信息。
	Extra map[string]any `json:"extra,omitempty"`
}

// MessageOutputImage is used to represent an image part in message.
//
// MessageOutputImage 用于表示消息中的图像部分。
type MessageOutputImage struct {
	MessagePartCommon
}

// MessageOutputAudio is used to represent an audio part in message.
//
// MessageOutputAudio 用于表示消息中的音频部分。
type MessageOutputAudio struct {
	MessagePartCommon
}

// MessageOutputVideo is used to represent a video part in message.
//
// MessageOutputVideo 用于表示消息中的视频部分。
type MessageOutputVideo struct {
	MessagePartCommon
}

// MessageOutputPart represents a part of an assistant-generated message.
// It can contain text, or multimedia content like images, audio, or video.
//
// MessageOutputPart 表示助手生成的消息的一部分。
// 它可以包含文本或多媒体内容，如图像、音频或视频。
type MessageOutputPart struct {
	// Type is the type of the part, eg. "text", "image_url", "audio_url", "video_url".
	//
	// Type 是部分的类型，例如 "text", "image_url", "audio_url", "video_url"。
	Type ChatMessagePartType `json:"type"`

	// Text is the text of the part, it's used when Type is "text".
	//
	// Text 是部分的文本，当 Type 为 "text" 时使用。
	Text string `json:"text,omitempty"`

	// Image is the image output of the part, used when Type is ChatMessagePartTypeImageURL.
	//
	// Image 是部分的图像输出，当 Type 为 ChatMessagePartTypeImageURL 时使用。
	Image *MessageOutputImage `json:"image,omitempty"`

	// Audio is the audio output of the part, used when Type is ChatMessagePartTypeAudioURL.
	//
	// Audio 是部分的音频输出，当 Type 为 ChatMessagePartTypeAudioURL 时使用。
	Audio *MessageOutputAudio `json:"audio,omitempty"`

	// Video is the video output of the part, used when Type is ChatMessagePartTypeVideoURL.
	//
	// Video 是部分的视频输出，当 Type 为 ChatMessagePartTypeVideoURL 时使用。
	Video *MessageOutputVideo `json:"video,omitempty"`

	// Extra is used to store extra information.
	//
	// Extra 用于存储额外信息。
	Extra map[string]any `json:"extra,omitempty"`
}

// Deprecated: This struct is deprecated as the MultiContent field is deprecated.
// For the image input part of the model, use MessageInputImage.
// For the image output part of the model, use MessageOutputImage.
// Choose either URL or URI.
// If your model implementation supports it, URL could embed inline image data
// as defined in RFC-2397.
//
// Deprecated: 由于 MultiContent 字段已弃用，此结构已弃用。
// 对于模型的图像输入部分，请使用 MessageInputImage。
// 对于模型的图像输出部分，请使用 MessageOutputImage。
// 选择 URL 或 URI。
// 如果您的模型实现支持，URL 可以嵌入 RFC-2397 定义的内联图像数据。
type ChatMessageImageURL struct {
	// URL can either be a traditional URL or a special URL conforming to RFC-2397 (https://www.rfc-editor.org/rfc/rfc2397).
	// double check with model implementations for detailed instructions on how to use this.
	//
	// URL 可以是传统 URL 或符合 RFC-2397 的特殊 URL。
	// 请与模型实现进行详细检查以获取有关如何使用此功能的说明。
	URL string `json:"url,omitempty"`

	URI string `json:"uri,omitempty"`
	// Detail is the quality of the image url.
	//
	// Detail 是图片 url 的质量。
	Detail ImageURLDetail `json:"detail,omitempty"`

	// MIMEType is the mime type of the image, eg. "image/png".
	//
	// MIMEType 是图像的 mime 类型，例如 "image/png"。
	MIMEType string `json:"mime_type,omitempty"`
	// Extra is used to store extra information for the image url.
	//
	// Extra 用于存储图片 url 的额外信息。
	Extra map[string]any `json:"extra,omitempty"`
}

// ChatMessagePartType is the type of the part in a chat message.
//
// ChatMessagePartType 是聊天消息中部分的类型。
type ChatMessagePartType string

const (
	// ChatMessagePartTypeText means the part is a text.
	//
	// ChatMessagePartTypeText 表示该部分是文本。
	ChatMessagePartTypeText ChatMessagePartType = "text"
	// ChatMessagePartTypeImageURL means the part is an image url.
	//
	// ChatMessagePartTypeImageURL 表示该部分是图片 url。
	ChatMessagePartTypeImageURL ChatMessagePartType = "image_url"
	// ChatMessagePartTypeAudioURL means the part is an audio url.
	//
	// ChatMessagePartTypeAudioURL 表示该部分是音频 url。
	ChatMessagePartTypeAudioURL ChatMessagePartType = "audio_url"
	// ChatMessagePartTypeVideoURL means the part is a video url.
	//
	// ChatMessagePartTypeVideoURL 表示该部分是视频 url。
	ChatMessagePartTypeVideoURL ChatMessagePartType = "video_url"
	// ChatMessagePartTypeFileURL means the part is a file url.
	//
	// ChatMessagePartTypeFileURL 表示该部分是文件 url。
	ChatMessagePartTypeFileURL ChatMessagePartType = "file_url"
)

// Deprecated: This struct is deprecated as the MultiContent field is deprecated.
// For the audio input part of the model, use MessageInputAudio.
// For the audio output part of the model, use MessageOutputAudio.
// Choose either URL or URI.
// If supported, URL may embed inline audio data per RFC-2397.
//
// Deprecated: 由于 MultiContent 字段已弃用，此结构已弃用。
// 对于模型的音频输入部分，请使用 MessageInputAudio。
// 对于模型的音频输出部分，请使用 MessageOutputAudio。
// 选择 URL 或 URI。
// 如果支持，URL 可以根据 RFC-2397 嵌入内联音频数据。
type ChatMessageAudioURL struct {
	// URL can either be a traditional URL or a special URL conforming to RFC-2397 (https://www.rfc-editor.org/rfc/rfc2397).
	// double check with model implementations for detailed instructions on how to use this.
	//
	// URL 可以是传统 URL 或符合 RFC-2397 的特殊 URL。
	// 请与模型实现进行详细检查以获取有关如何使用此功能的说明。
	URL string `json:"url,omitempty"`
	URI string `json:"uri,omitempty"`

	// MIMEType is the mime type of the audio, eg. "audio/wav" or "audio/ogg".
	//
	// MIMEType 是音频的 mime 类型，例如 "audio/wav" 或 "audio/ogg"。
	MIMEType string `json:"mime_type,omitempty"`
	// Extra is used to store extra information for the audio url.
	//
	// Extra 用于存储音频 url 的额外信息。
	Extra map[string]any `json:"extra,omitempty"`
}

// Deprecated: This struct is deprecated as the MultiContent field is deprecated.
// For the video input part of the model, use MessageInputVideo.
// For the video output part of the model, use MessageOutputVideo.
// Choose either URL or URI.
// If supported, URL may embed inline video data per RFC-2397.
//
// Deprecated: 由于 MultiContent 字段已弃用，此结构已弃用。
// 对于模型的视频输入部分，请使用 MessageInputVideo。
// 对于模型的视频输出部分，请使用 MessageOutputVideo。
// 选择 URL 或 URI。
// 如果支持，URL 可以根据 RFC-2397 嵌入内联视频数据。
type ChatMessageVideoURL struct {
	// URL can either be a traditional URL or a special URL conforming to RFC-2397 (https://www.rfc-editor.org/rfc/rfc2397).
	// double check with model implementations for detailed instructions on how to use this.
	//
	// URL 可以是传统 URL 或符合 RFC-2397 的特殊 URL。
	// 请与模型实现进行详细检查以获取有关如何使用此功能的说明。
	URL string `json:"url,omitempty"`
	URI string `json:"uri,omitempty"`

	// MIMEType is the mime type of the video, eg. "video/mp4".
	//
	// MIMEType 是视频的 mime 类型，例如 "video/mp4"。
	MIMEType string `json:"mime_type,omitempty"`
	// Extra is used to store extra information for the video url.
	//
	// Extra 用于存储视频 url 的额外信息。
	Extra map[string]any `json:"extra,omitempty"`
}

// Deprecated: This struct is deprecated as the MultiContent field is deprecated.
// For the file input part of the model, use MessageInputFile.
// Choose either URL or URI.
//
// Deprecated: 由于 MultiContent 字段已弃用，此结构已弃用。
// 对于模型的文件输入部分，请使用 MessageInputFile。
// 选择 URL 或 URI。
type ChatMessageFileURL struct {
	URL string `json:"url,omitempty"`
	URI string `json:"uri,omitempty"`

	// MIMEType is the mime type of the file, eg. "application/pdf", "text/plain".
	//
	// MIMEType 是文件的 mime 类型，例如 "application/pdf", "text/plain"。
	MIMEType string `json:"mime_type,omitempty"`
	// Name is the name of the file.
	//
	// Name 是文件名。
	Name string `json:"name,omitempty"`

	// Extra is used to store extra information for the file url.
	//
	// Extra 用于存储文件 url 的额外信息。
	Extra map[string]any `json:"extra,omitempty"`
}

// Deprecated: This struct is deprecated as the MultiContent field is deprecated.
// For model input, use MessageInputPart. For model output, use MessageOutputPart.
//
// Deprecated: 由于 MultiContent 字段已弃用，此结构已弃用。
// 对于模型输入，请使用 MessageInputPart。对于模型输出，请使用 MessageOutputPart。
type ChatMessagePart struct {
	// Type is the type of the part, eg. "text", "image_url", "audio_url", "video_url", "file_url".
	//
	// Type 是部分的类型，例如 "text", "image_url", "audio_url", "video_url", "file_url"。
	Type ChatMessagePartType `json:"type,omitempty"`

	// Text is the text of the part, it's used when Type is "text".
	//
	// Text 是部分的文本，当 Type 为 "text" 时使用。
	Text string `json:"text,omitempty"`

	// ImageURL is the image url of the part, it's used when Type is "image_url".
	//
	// ImageURL 是部分的图片 url，当 Type 为 "image_url" 时使用。
	ImageURL *ChatMessageImageURL `json:"image_url,omitempty"`
	// AudioURL is the audio url of the part, it's used when Type is "audio_url".
	//
	// AudioURL 是部分的音频 url，当 Type 为 "audio_url" 时使用。
	AudioURL *ChatMessageAudioURL `json:"audio_url,omitempty"`
	// VideoURL is the video url of the part, it's used when Type is "video_url".
	//
	// VideoURL 是部分的视频 url，当 Type 为 "video_url" 时使用。
	VideoURL *ChatMessageVideoURL `json:"video_url,omitempty"`
	// FileURL is the file url of the part, it's used when Type is "file_url".
	//
	// FileURL 是部分的文件 url，当 Type 为 "file_url" 时使用。
	FileURL *ChatMessageFileURL `json:"file_url,omitempty"`
}

// LogProbs is the top-level structure containing the log probability information.
//
// LogProbs 是包含对数概率信息的顶级结构。
type LogProbs struct {
	// Content is a list of message content tokens with log probability information.
	//
	// Content 是带有对数概率信息的消息内容标记列表。
	Content []LogProb `json:"content"`
}

// LogProb represents the probability information for a token.
//
// LogProb 表示标记的概率信息。
type LogProb struct {
	// Token represents the text of the token, which is a contiguous sequence of characters
	// (e.g., a word, part of a word, or punctuation) as understood by the tokenization process used by the language model.
	//
	// Token 表示标记的文本，它是语言模型使用的标记化过程所理解的连续字符序列（例如，单词、单词的一部分或标点符号）。
	Token string `json:"token"`
	// LogProb is the log probability of this token, if it is within the top 20 most likely tokens.
	// Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.
	//
	// LogProb 是此标记的对数概率（如果它位于前 20 个最可能的标记中）。
	// 否则，使用值 `-9999.0` 表示该标记不太可能。
	LogProb float64 `json:"logprob"`
	// Bytes is a list of integers representing the UTF-8 bytes representation of the token.
	// Useful in instances where characters are represented by multiple tokens and
	// their byte representations must be combined to generate the correct text
	// representation. Can be `null` if there is no bytes representation for the token.
	//
	// Bytes 是表示标记的 UTF-8 字节表示形式的整数列表。
	// 在字符由多个标记表示且必须组合其字节表示形式以生成正确文本表示形式的情况下很有用。
	// 如果标记没有字节表示形式，则可以为 `null`。
	Bytes []int64 `json:"bytes,omitempty"` // Omitting the field if it is null
	// TopLogProbs is a list of the most likely tokens and their log probability, at this token position.
	// In rare cases, there may be fewer than the number of requested top_logprobs returned.
	//
	// TopLogProbs 是此标记位置最可能的标记及其对数概率的列表。
	// 在极少数情况下，返回的 top_logprobs 数量可能少于请求的数量。
	TopLogProbs []TopLogProb `json:"top_logprobs"`
}

// TopLogProb describes a likely token and its log probability at a position.
//
// TopLogProb 描述了某个位置可能的标记及其对数概率。
type TopLogProb struct {
	// Token represents the text of the token, which is a contiguous sequence of characters
	// (e.g., a word, part of a word, or punctuation) as understood by the tokenization process used by the language model.
	//
	// Token 表示标记的文本，它是语言模型使用的标记化过程所理解的连续字符序列（例如，单词、单词的一部分或标点符号）。
	Token string `json:"token"`
	// LogProb is the log probability of this token, if it is within the top 20 most likely tokens.
	// Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.
	//
	// LogProb 是此标记的对数概率（如果它位于前 20 个最可能的标记中）。
	// 否则，使用值 `-9999.0` 表示该标记不太可能。
	LogProb float64 `json:"logprob"`
	// Bytes is a list of integers representing the UTF-8 bytes representation of the token.
	// Useful in instances where characters are represented by multiple tokens and
	// their byte representations must be combined to generate the correct text
	// representation. Can be `null` if there is no bytes representation for the token.
	//
	// Bytes 是表示标记的 UTF-8 字节表示形式的整数列表。
	// 在字符由多个标记表示且必须组合其字节表示形式以生成正确文本表示形式的情况下很有用。
	// 如果标记没有字节表示形式，则可以为 `null`。
	Bytes []int64 `json:"bytes,omitempty"`
}

// ResponseMeta collects meta information about a chat response.
//
// ResponseMeta 收集有关聊天响应的元信息。
type ResponseMeta struct {
	// FinishReason is the reason why the chat response is finished.
	// It's usually "stop", "length", "tool_calls", "content_filter", "null". This is defined by chat model implementation.
	//
	// FinishReason 是聊天响应结束的原因。
	// 通常是 "stop", "length", "tool_calls", "content_filter", "null"。这是由聊天模型实现定义的。
	FinishReason string `json:"finish_reason,omitempty"`
	// Usage is the token usage of the chat response, whether usage exists depends on whether the chat model implementation returns.
	//
	// Usage 是聊天响应的标记使用情况，是否存在取决于聊天模型实现是否返回。
	Usage *TokenUsage `json:"usage,omitempty"`
	// LogProbs is Log probability information.
	//
	// LogProbs 是对数概率信息。
	LogProbs *LogProbs `json:"logprobs,omitempty"`
}

// Message denotes the data structure for model input and output, originating from either user input or model return.
// It supports both text-only and multimodal content.
//
// For text-only input from a user, use the Content field:
//
//	&schema.Message{
//		Role:    schema.User,
//		Content: "What is the capital of France?",
//	}
//
// For multimodal input from a user, use the UserInputMultiContent field.
// This allows combining text with other media like images:
//
//	&schema.Message{
//		Role: schema.User,
//		UserInputMultiContent: []schema.MessageInputPart{
//			{Type: schema.ChatMessagePartTypeText, Text: "What is in this image?"},
//			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
//				MessagePartCommon: schema.MessagePartCommon{
//					URL: toPtr("https://example.com/cat.jpg"),
//				},
//				Detail: schema.ImageURLDetailHigh,
//			}},
//		},
//	}
//
// When the model returns multimodal content, it is available in the AssistantGenMultiContent field:
//
//	&schema.Message{
//		Role: schema.Assistant,
//		AssistantGenMultiContent: []schema.MessageOutputPart{
//			{Type: schema.ChatMessagePartTypeText, Text: "Here is the generated image:"},
//			{Type: schema.ChatMessagePartTypeImage, Image: &schema.MessageOutputImage{
//				MessagePartCommon: schema.MessagePartCommon{
//					Base64Data: toPtr("base64_image_binary"),
//					MIMEType:   "image/png",
//				},
//			}},
//		},
//	}
//
// Message 表示模型输入和输出的数据结构，源自用户输入或模型返回。
// 它支持纯文本和多模态内容。
//
// 对于来自用户的纯文本输入，请使用 Content 字段：
//
//	&schema.Message{
//		Role:    schema.User,
//		Content: "What is the capital of France?",
//	}
//
// 对于来自用户的多模态输入，请使用 UserInputMultiContent 字段。
// 这允许将文本与其他媒体（如图像）结合使用：
//
//	&schema.Message{
//		Role: schema.User,
//		UserInputMultiContent: []schema.MessageInputPart{
//			{Type: schema.ChatMessagePartTypeText, Text: "What is in this image?"},
//			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
//				MessagePartCommon: schema.MessagePartCommon{
//					URL: toPtr("https://example.com/cat.jpg"),
//				},
//				Detail: schema.ImageURLDetailHigh,
//			}},
//		},
//	}
//
// 当模型返回多模态内容时，它在 AssistantGenMultiContent 字段中可用：
//
//	&schema.Message{
//		Role: schema.Assistant,
//		AssistantGenMultiContent: []schema.MessageOutputPart{
//			{Type: schema.ChatMessagePartTypeText, Text: "Here is the generated image:"},
//			{Type: schema.ChatMessagePartTypeImage, Image: &schema.MessageOutputImage{
//				MessagePartCommon: schema.MessagePartCommon{
//					Base64Data: toPtr("base64_image_binary"),
//					MIMEType:   "image/png",
//				},
//			}},
//		},
//	}
type Message struct {
	Role RoleType `json:"role"`

	// Content is for user text input and model text output.
	//
	// Content 用于用户文本输入和模型文本输出。
	Content string `json:"content"`

	// if MultiContent is not empty, use this instead of Content
	// if MultiContent is empty, use Content
	// Deprecated: Use UserInputMultiContent for user multimodal inputs and AssistantGenMultiContent for model multimodal outputs.
	//
	// Deprecated: 请使用 UserInputMultiContent 用于用户多模态输入，使用 AssistantGenMultiContent 用于模型多模态输出。
	MultiContent []ChatMessagePart `json:"multi_content,omitempty"`

	// UserInputMultiContent passes multimodal content provided by the user to the model.
	//
	// UserInputMultiContent 将用户提供的多模态内容传递给模型。
	UserInputMultiContent []MessageInputPart `json:"user_input_multi_content,omitempty"`

	// AssistantGenMultiContent is for receiving multimodal output from the model.
	//
	// AssistantGenMultiContent 用于接收模型的具体多模态输出。
	AssistantGenMultiContent []MessageOutputPart `json:"assistant_output_multi_content,omitempty"`

	Name string `json:"name,omitempty"`

	// only for AssistantMessage
	//
	// 仅用于 AssistantMessage
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// only for ToolMessage
	//
	// 仅用于 ToolMessage
	ToolCallID string `json:"tool_call_id,omitempty"`
	// only for ToolMessage
	//
	// 仅用于 ToolMessage
	ToolName string `json:"tool_name,omitempty"`

	ResponseMeta *ResponseMeta `json:"response_meta,omitempty"`

	// ReasoningContent is the thinking process of the model, which will be included when the model returns reasoning content.
	//
	// ReasoningContent 是模型的思考过程，当模型返回推理内容时将包含此内容。
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// customized information for model implementation
	//
	// 模型实现的自定义信息
	Extra map[string]any `json:"extra,omitempty"`
}

// TokenUsage Represents the token usage of chat model request.
//
// TokenUsage 表示聊天模型请求的标记使用情况。
type TokenUsage struct {
	// PromptTokens is the number of prompt tokens, including all the input tokens of this request.
	//
	// PromptTokens 是提示标记的数量，包括此请求的所有输入标记。
	PromptTokens int `json:"prompt_tokens"`
	// PromptTokenDetails is a breakdown of the prompt tokens.
	//
	// PromptTokenDetails 是提示标记的细分。
	PromptTokenDetails PromptTokenDetails `json:"prompt_token_details"`
	// CompletionTokens is the number of completion tokens.
	//
	// CompletionTokens 是完成标记的数量。
	CompletionTokens int `json:"completion_tokens"`
	// TotalTokens is the total number of tokens.
	//
	// TotalTokens 是标记的总数。
	TotalTokens int `json:"total_tokens"`
	// CompletionTokensDetails is breakdown of completion tokens.
	//
	// CompletionTokensDetails 是完成标记的细分。
	CompletionTokensDetails CompletionTokensDetails `json:"completion_token_details"`
}

type CompletionTokensDetails struct {
	// ReasoningTokens tokens generated by the model for reasoning.
	// This is currently supported by OpenAI, Gemini, ARK and Qwen  chat models.
	// For other models, this field will be 0.
	//
	// ReasoningTokens 模型生成的用于推理的标记。
	// 目前 OpenAI, Gemini, ARK 和 Qwen 聊天模型支持此功能。
	// 对于其他模型，此字段将为 0。
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// PromptTokenDetails provides a breakdown of prompt token usage.
//
// PromptTokenDetails 提供提示标记使用情况的细分。
type PromptTokenDetails struct {
	// Cached tokens present in the prompt.
	//
	// CachedTokens 提示中存在的缓存标记。
	CachedTokens int `json:"cached_tokens"`
}

var _ MessagesTemplate = &Message{}
var _ MessagesTemplate = MessagesPlaceholder("", false)

// MessagesTemplate is the interface for messages template.
// It's used to render a template to a list of messages.
// e.g.
//
//	chatTemplate := prompt.FromMessages(
//		schema.SystemMessage("you are eino helper"),
//		schema.MessagesPlaceholder("history", false), // <= this will use the value of "history" in params
//	)
//	msgs, err := chatTemplate.Format(ctx, params)
//
// MessagesTemplate 是消息模板的接口。
// 它用于将模板渲染为消息列表。
// 例如：
//
//	chatTemplate := prompt.FromMessages(
//		schema.SystemMessage("you are eino helper"),
//		schema.MessagesPlaceholder("history", false), // <= this will use the value of "history" in params
//	)
//	msgs, err := chatTemplate.Format(ctx, params)
type MessagesTemplate interface {
	Format(ctx context.Context, vs map[string]any, formatType FormatType) ([]*Message, error)
}

type messagesPlaceholder struct {
	key      string
	optional bool
}

// MessagesPlaceholder can render a placeholder to a list of messages in params.
// e.g.
//
//	placeholder := MessagesPlaceholder("history", false)
//	params := map[string]any{
//		"history": []*schema.Message{{Role: "user", Content: "what is eino?"}, {Role: "assistant", Content: "eino is a great freamwork to build llm apps"}},
//		"query": "how to use eino?",
//	}
//	chatTemplate := chatTpl := prompt.FromMessages(
//		schema.SystemMessage("you are eino helper"),
//		schema.MessagesPlaceholder("history", false), // <= this will use the value of "history" in params
//	)
//	msgs, err := chatTemplate.Format(ctx, params)
//
// MessagesPlaceholder 可以将占位符渲染为 params 中的消息列表。
// 例如：
//
//	placeholder := MessagesPlaceholder("history", false)
//	params := map[string]any{
//		"history": []*schema.Message{{Role: "user", Content: "what is eino?"}, {Role: "assistant", Content: "eino is a great freamwork to build llm apps"}},
//		"query": "how to use eino?",
//	}
//	chatTemplate := chatTpl := prompt.FromMessages(
//		schema.SystemMessage("you are eino helper"),
//		schema.MessagesPlaceholder("history", false), // <= this will use the value of "history" in params
//	)
//	msgs, err := chatTemplate.Format(ctx, params)
func MessagesPlaceholder(key string, optional bool) MessagesTemplate {
	return &messagesPlaceholder{
		key:      key,
		optional: optional,
	}
}

// Format just return the messages of specified key.
// because it's a placeholder.
// e.g.
//
//	placeholder := MessagesPlaceholder("history", false)
//	params := map[string]any{
//		"history": []*schema.Message{{Role: "user", Content: "what is eino?"}, {Role: "assistant", Content: "eino is a great freamwork to build llm apps"}},
//		"query": "how to use eino?",
//	}
//	msgs, err := placeholder.Format(ctx, params) // <= this will return the value of "history" in params
//
// Format 仅返回指定 key 的消息。
// 因为它是一个占位符。
// 例如：
//
//	placeholder := MessagesPlaceholder("history", false)
//	params := map[string]any{
//		"history": []*schema.Message{{Role: "user", Content: "what is eino?"}, {Role: "assistant", Content: "eino is a great freamwork to build llm apps"}},
//		"query": "how to use eino?",
//	}
//	msgs, err := placeholder.Format(ctx, params) // <= this will return the value of "history" in params
func (p *messagesPlaceholder) Format(_ context.Context, vs map[string]any, _ FormatType) ([]*Message, error) {
	v, ok := vs[p.key]
	if !ok {
		if p.optional {
			return []*Message{}, nil
		}

		return nil, fmt.Errorf("message placeholder format: %s not found", p.key)
	}

	msgs, ok := v.([]*Message)
	if !ok {
		return nil, fmt.Errorf("only messages can be used to format message placeholder, key: %v, actual type: %v", p.key, reflect.TypeOf(v))
	}

	return msgs, nil
}

func formatContent(content string, vs map[string]any, formatType FormatType) (string, error) {
	switch formatType {
	case FString:
		return pyfmt.Fmt(content, vs)
	case GoTemplate:
		parsedTmpl, err := template.New("template").
			Option("missingkey=error").
			Parse(content)
		if err != nil {
			return "", err
		}
		sb := new(strings.Builder)
		err = parsedTmpl.Execute(sb, vs)
		if err != nil {
			return "", err
		}
		return sb.String(), nil
	case Jinja2:
		env, err := getJinjaEnv()
		if err != nil {
			return "", err
		}
		tpl, err := env.FromString(content)
		if err != nil {
			return "", err
		}
		out, err := tpl.Execute(vs)
		if err != nil {
			return "", err
		}
		return out, nil
	default:
		return "", fmt.Errorf("unknown format type: %v", formatType)
	}
}

// Format returns the messages after rendering by the given formatType.
// e.g.
//
//	msg := schema.UserMessage("hello world, {name}")
//	msgs, err := msg.Format(ctx, map[string]any{"name": "eino"}, schema.FString) // <= this will render the content of msg by pyfmt
//	// msgs[0].Content will be "hello world, eino"
//
// Format 返回按给定 formatType 渲染后的消息。
// 例如：
//
//	msg := schema.UserMessage("hello world, {name}")
//	msgs, err := msg.Format(ctx, map[string]any{"name": "eino"}, schema.FString) // <= 这将使用 pyfmt 渲染 msg 的内容
//	// msgs[0].Content 将是 "hello world, eino"
func (m *Message) Format(_ context.Context, vs map[string]any, formatType FormatType) ([]*Message, error) {
	c, err := formatContent(m.Content, vs, formatType)
	if err != nil {
		return nil, err
	}
	copied := *m
	copied.Content = c

	if len(m.MultiContent) > 0 {
		copied.MultiContent, err = formatMultiContent(m.MultiContent, vs, formatType)
		if err != nil {
			return nil, err
		}
	}

	if len(m.UserInputMultiContent) > 0 {
		copied.UserInputMultiContent, err = formatUserInputMultiContent(m.UserInputMultiContent, vs, formatType)
		if err != nil {
			return nil, err
		}
	}

	return []*Message{&copied}, nil
}

func formatMultiContent(multiContent []ChatMessagePart, vs map[string]any, formatType FormatType) ([]ChatMessagePart, error) {
	copiedMC := make([]ChatMessagePart, len(multiContent))
	copy(copiedMC, multiContent)

	for i, mc := range copiedMC {
		switch mc.Type {
		case ChatMessagePartTypeText:
			nmc, err := formatContent(mc.Text, vs, formatType)
			if err != nil {
				return nil, err
			}
			copiedMC[i].Text = nmc
		case ChatMessagePartTypeImageURL:
			if mc.ImageURL == nil {
				continue
			}
			url, err := formatContent(mc.ImageURL.URL, vs, formatType)
			if err != nil {
				return nil, err
			}
			copiedMC[i].ImageURL.URL = url
		case ChatMessagePartTypeAudioURL:
			if mc.AudioURL == nil {
				continue
			}
			url, err := formatContent(mc.AudioURL.URL, vs, formatType)
			if err != nil {
				return nil, err
			}
			copiedMC[i].AudioURL.URL = url
		case ChatMessagePartTypeVideoURL:
			if mc.VideoURL == nil {
				continue
			}
			url, err := formatContent(mc.VideoURL.URL, vs, formatType)
			if err != nil {
				return nil, err
			}
			copiedMC[i].VideoURL.URL = url
		case ChatMessagePartTypeFileURL:
			if mc.FileURL == nil {
				continue
			}
			url, err := formatContent(mc.FileURL.URL, vs, formatType)
			if err != nil {
				return nil, err
			}
			copiedMC[i].FileURL.URL = url
		}
	}

	return copiedMC, nil
}

func formatUserInputMultiContent(userInputMultiContent []MessageInputPart, vs map[string]any, formatType FormatType) ([]MessageInputPart, error) {
	copiedUIMC := make([]MessageInputPart, len(userInputMultiContent))
	copy(copiedUIMC, userInputMultiContent)

	for i, uimc := range copiedUIMC {
		switch uimc.Type {
		case ChatMessagePartTypeText:
			text, err := formatContent(uimc.Text, vs, formatType)
			if err != nil {
				return nil, err
			}
			copiedUIMC[i].Text = text
		case ChatMessagePartTypeImageURL:
			if uimc.Image == nil {
				continue
			}
			if uimc.Image.URL != nil && *uimc.Image.URL != "" {
				url, err := formatContent(*uimc.Image.URL, vs, formatType)
				if err != nil {
					return nil, err
				}
				copiedUIMC[i].Image.URL = &url
			}
			if uimc.Image.Base64Data != nil && *uimc.Image.Base64Data != "" {
				base64data, err := formatContent(*uimc.Image.Base64Data, vs, formatType)
				if err != nil {
					return nil, err
				}
				copiedUIMC[i].Image.Base64Data = &base64data
			}
		case ChatMessagePartTypeAudioURL:
			if uimc.Audio == nil {
				continue
			}
			if uimc.Audio.URL != nil && *uimc.Audio.URL != "" {
				url, err := formatContent(*uimc.Audio.URL, vs, formatType)
				if err != nil {
					return nil, err
				}
				copiedUIMC[i].Audio.URL = &url
			}
			if uimc.Audio.Base64Data != nil && *uimc.Audio.Base64Data != "" {
				base64data, err := formatContent(*uimc.Audio.Base64Data, vs, formatType)
				if err != nil {
					return nil, err
				}
				copiedUIMC[i].Audio.Base64Data = &base64data
			}
		case ChatMessagePartTypeVideoURL:
			if uimc.Video == nil {
				continue
			}
			if uimc.Video.URL != nil && *uimc.Video.URL != "" {
				url, err := formatContent(*uimc.Video.URL, vs, formatType)
				if err != nil {
					return nil, err
				}
				copiedUIMC[i].Video.URL = &url
			}
			if uimc.Video.Base64Data != nil && *uimc.Video.Base64Data != "" {
				base64data, err := formatContent(*uimc.Video.Base64Data, vs, formatType)
				if err != nil {
					return nil, err
				}
				copiedUIMC[i].Video.Base64Data = &base64data
			}
		case ChatMessagePartTypeFileURL:
			if uimc.File == nil {
				continue
			}
			if uimc.File.URL != nil && *uimc.File.URL != "" {
				url, err := formatContent(*uimc.File.URL, vs, formatType)
				if err != nil {
					return nil, err
				}
				copiedUIMC[i].File.URL = &url
			}
			if uimc.File.Base64Data != nil && *uimc.File.Base64Data != "" {
				base64data, err := formatContent(*uimc.File.Base64Data, vs, formatType)
				if err != nil {
					return nil, err
				}
				copiedUIMC[i].File.Base64Data = &base64data
			}
		}
	}

	return copiedUIMC, nil
}

// String returns the string representation of the message.
// e.g.
//
//	msg := schema.UserMessage("hello world")
//	fmt.Println(msg.String()) // Output will be: `user: hello world``
//
//	msg := schema.Message{
//		Role:    schema.Tool,
//		Content: "{...}",
//		ToolCallID: "callxxxx"
//	}
//	fmt.Println(msg.String())
//	Output will be:
//		tool: {...}
//		call_id: callxxxx
func (m *Message) String() string {
	sb := &strings.Builder{}
	sb.WriteString(fmt.Sprintf("%s: %s", m.Role, m.Content))
	if len(m.ReasoningContent) > 0 {
		sb.WriteString("\nreasoning content:\n")
		sb.WriteString(m.ReasoningContent)
	}
	if len(m.ToolCalls) > 0 {
		sb.WriteString("\ntool_calls:\n")
		for _, tc := range m.ToolCalls {
			if tc.Index != nil {
				sb.WriteString(fmt.Sprintf("index[%d]:", *tc.Index))
			}
			sb.WriteString(fmt.Sprintf("%+v\n", tc))
		}
	}
	if m.ToolCallID != "" {
		sb.WriteString(fmt.Sprintf("\ntool_call_id: %s", m.ToolCallID))
	}
	if m.ToolName != "" {
		sb.WriteString(fmt.Sprintf("\ntool_call_name: %s", m.ToolName))
	}
	if m.ResponseMeta != nil {
		sb.WriteString(fmt.Sprintf("\nfinish_reason: %s", m.ResponseMeta.FinishReason))
		if m.ResponseMeta.Usage != nil {
			sb.WriteString(fmt.Sprintf("\nusage: %v", m.ResponseMeta.Usage))
		}
	}

	return sb.String()
}

// SystemMessage represents a message with Role "system".
func SystemMessage(content string) *Message {
	return &Message{
		Role:    System,
		Content: content,
	}
}

// AssistantMessage represents a message with Role "assistant".
func AssistantMessage(content string, toolCalls []ToolCall) *Message {
	return &Message{
		Role:      Assistant,
		Content:   content,
		ToolCalls: toolCalls,
	}
}

// UserMessage represents a message with Role "user".
func UserMessage(content string) *Message {
	return &Message{
		Role:    User,
		Content: content,
	}

}

type toolMessageOptions struct {
	toolName string
}

// ToolMessageOption defines a option for ToolMessage
type ToolMessageOption func(*toolMessageOptions)

// WithToolName returns a ToolMessageOption that sets the tool call name.
func WithToolName(name string) ToolMessageOption {
	return func(o *toolMessageOptions) {
		o.toolName = name
	}
}

// ToolMessage represents a message with Role "tool".
func ToolMessage(content string, toolCallID string, opts ...ToolMessageOption) *Message {
	o := &toolMessageOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return &Message{
		Role:       Tool,
		Content:    content,
		ToolCallID: toolCallID,
		ToolName:   o.toolName,
	}
}

func concatToolCalls(chunks []ToolCall) ([]ToolCall, error) {
	var merged []ToolCall
	m := make(map[int][]int)
	for i := range chunks {
		index := chunks[i].Index
		if index == nil {
			merged = append(merged, chunks[i])
		} else {
			m[*index] = append(m[*index], i)
		}
	}

	var args strings.Builder
	for k, v := range m {
		index := k
		toolCall := ToolCall{Index: &index}
		if len(v) > 0 {
			toolCall = chunks[v[0]]
		}

		args.Reset()
		toolID, toolType, toolName := "", "", "" // these field will output atomically in any chunk

		for _, n := range v {
			chunk := chunks[n]
			if chunk.ID != "" {
				if toolID == "" {
					toolID = chunk.ID
				} else if toolID != chunk.ID {
					return nil, fmt.Errorf("cannot concat ToolCalls with different tool id: '%s' '%s'", toolID, chunk.ID)
				}

			}

			if chunk.Type != "" {
				if toolType == "" {
					toolType = chunk.Type
				} else if toolType != chunk.Type {
					return nil, fmt.Errorf("cannot concat ToolCalls with different tool type: '%s' '%s'", toolType, chunk.Type)
				}
			}

			if chunk.Function.Name != "" {
				if toolName == "" {
					toolName = chunk.Function.Name
				} else if toolName != chunk.Function.Name {
					return nil, fmt.Errorf("cannot concat ToolCalls with different tool name: '%s' '%s'", toolName, chunk.Function.Name)
				}
			}

			if chunk.Function.Arguments != "" {
				_, err := args.WriteString(chunk.Function.Arguments)
				if err != nil {
					return nil, err
				}
			}
		}

		toolCall.ID = toolID
		toolCall.Type = toolType
		toolCall.Function.Name = toolName
		toolCall.Function.Arguments = args.String()

		merged = append(merged, toolCall)
	}

	if len(merged) > 1 {
		sort.SliceStable(merged, func(i, j int) bool {
			iVal, jVal := merged[i].Index, merged[j].Index
			if iVal == nil && jVal == nil {
				return false
			} else if iVal == nil && jVal != nil {
				return true
			} else if iVal != nil && jVal == nil {
				return false
			}

			return *iVal < *jVal
		})
	}

	return merged, nil
}

func isBase64AudioPart(part MessageOutputPart) bool {
	return part.Type == ChatMessagePartTypeAudioURL &&
		part.Audio != nil &&
		part.Audio.Base64Data != nil &&
		part.Audio.URL == nil
}

func concatAssistantMultiContent(parts []MessageOutputPart) ([]MessageOutputPart, error) {
	if len(parts) == 0 {
		return parts, nil
	}

	merged := make([]MessageOutputPart, 0, len(parts))
	i := 0
	for i < len(parts) {
		currentPart := parts[i]
		start := i

		if currentPart.Type == ChatMessagePartTypeText {
			// --- Text Merging ---
			// Find end of contiguous text block
			end := start + 1
			for end < len(parts) && parts[end].Type == ChatMessagePartTypeText {
				end++
			}

			// If only one part, just append it
			if end == start+1 {
				merged = append(merged, currentPart)
			} else {
				// Multiple parts to merge
				var sb strings.Builder
				for k := start; k < end; k++ {
					sb.WriteString(parts[k].Text)
				}
				mergedPart := MessageOutputPart{
					Type: ChatMessagePartTypeText,
					Text: sb.String(),
				}
				merged = append(merged, mergedPart)
			}
			i = end
		} else if isBase64AudioPart(currentPart) {
			// --- Audio Merging ---
			// Find end of contiguous audio block
			end := start + 1
			for end < len(parts) && isBase64AudioPart(parts[end]) {
				end++
			}

			// If only one part, just append it
			if end == start+1 {
				merged = append(merged, currentPart)
			} else {
				// Multiple parts to merge
				var b64Builder strings.Builder
				var mimeType string
				extraList := make([]map[string]any, 0, end-start)

				for k := start; k < end; k++ {
					audioPart := parts[k].Audio
					if audioPart.Base64Data != nil {
						b64Builder.WriteString(*audioPart.Base64Data)
					}
					if mimeType == "" {
						mimeType = audioPart.MIMEType
					}
					if len(audioPart.Extra) > 0 {
						extraList = append(extraList, audioPart.Extra)
					}
				}

				var mergedExtra map[string]any
				var err error
				if len(extraList) > 0 {
					mergedExtra, err = concatExtra(extraList)
					if err != nil {
						return nil, fmt.Errorf("failed to concat audio extra: %w", err)
					}
				}

				mergedB64 := b64Builder.String()
				mergedPart := MessageOutputPart{
					Type: ChatMessagePartTypeAudioURL,
					Audio: &MessageOutputAudio{
						MessagePartCommon: MessagePartCommon{
							Base64Data: &mergedB64,
							MIMEType:   mimeType,
							Extra:      mergedExtra,
						},
					},
				}
				merged = append(merged, mergedPart)
			}
			i = end
		} else {
			// --- Non-mergeable part ---
			merged = append(merged, currentPart)
			i++
		}
	}

	return merged, nil
}

func concatExtra(extraList []map[string]any) (map[string]any, error) {
	if len(extraList) == 1 {
		return generic.CopyMap(extraList[0]), nil
	}

	return internal.ConcatItems(extraList)
}

// ConcatMessages concat messages with the same role and name.
// It will concat tool calls with the same index.
// It will return an error if the messages have different roles or names.
// It's useful for concatenating messages from a stream.
// e.g.
//
//	msgs := []*Message{}
//	for {
//		msg, err := stream.Recv()
//		if errors.Is(err, io.EOF) {
//			break
//		}
//		if err != nil {...}
//		msgs = append(msgs, msg)
//	}
//
// concatedMsg, err := ConcatMessages(msgs) // concatedMsg.Content will be full content of all messages
func ConcatMessages(msgs []*Message) (*Message, error) {
	var (
		contents                      []string
		contentLen                    int
		reasoningContents             []string
		reasoningContentLen           int
		toolCalls                     []ToolCall
		multiContentParts             []ChatMessagePart
		assistantGenMultiContentParts []MessageOutputPart
		ret                           = Message{}
		extraList                     = make([]map[string]any, 0, len(msgs))
	)

	for idx, msg := range msgs {
		if msg == nil {
			return nil, fmt.Errorf("unexpected nil chunk in message stream, index: %d", idx)
		}

		if msg.Role != "" {
			if ret.Role == "" {
				ret.Role = msg.Role
			} else if ret.Role != msg.Role {
				return nil, fmt.Errorf("cannot concat messages with "+
					"different roles: '%s' '%s'", ret.Role, msg.Role)
			}
		}

		if msg.Name != "" {
			if ret.Name == "" {
				ret.Name = msg.Name
			} else if ret.Name != msg.Name {
				return nil, fmt.Errorf("cannot concat messages with"+
					" different names: '%s' '%s'", ret.Name, msg.Name)
			}
		}

		if msg.ToolCallID != "" {
			if ret.ToolCallID == "" {
				ret.ToolCallID = msg.ToolCallID
			} else if ret.ToolCallID != msg.ToolCallID {
				return nil, fmt.Errorf("cannot concat messages with"+
					" different toolCallIDs: '%s' '%s'", ret.ToolCallID, msg.ToolCallID)
			}
		}
		if msg.ToolName != "" {
			if ret.ToolName == "" {
				ret.ToolName = msg.ToolName
			} else if ret.ToolName != msg.ToolName {
				return nil, fmt.Errorf("cannot concat messages with"+
					" different toolNames: '%s' '%s'", ret.ToolCallID, msg.ToolCallID)
			}
		}

		if msg.Content != "" {
			contents = append(contents, msg.Content)
			contentLen += len(msg.Content)
		}
		if msg.ReasoningContent != "" {
			reasoningContents = append(reasoningContents, msg.ReasoningContent)
			reasoningContentLen += len(msg.ReasoningContent)
		}

		if len(msg.ToolCalls) > 0 {
			toolCalls = append(toolCalls, msg.ToolCalls...)
		}

		if len(msg.Extra) > 0 {
			extraList = append(extraList, msg.Extra)
		}

		// The 'MultiContent' field is deprecated but is kept for backward compatibility.
		if len(msg.MultiContent) > 0 {
			multiContentParts = append(multiContentParts, msg.MultiContent...)
		}

		if len(msg.AssistantGenMultiContent) > 0 {
			assistantGenMultiContentParts = append(assistantGenMultiContentParts, msg.AssistantGenMultiContent...)
		}

		if msg.ResponseMeta != nil && ret.ResponseMeta == nil {
			ret.ResponseMeta = &ResponseMeta{}
		}

		if msg.ResponseMeta != nil && ret.ResponseMeta != nil {
			// keep the last FinishReason with a valid value.
			if msg.ResponseMeta.FinishReason != "" {
				ret.ResponseMeta.FinishReason = msg.ResponseMeta.FinishReason
			}

			if msg.ResponseMeta.Usage != nil {
				if ret.ResponseMeta.Usage == nil {
					ret.ResponseMeta.Usage = &TokenUsage{}
				}

				if msg.ResponseMeta.Usage.PromptTokens > ret.ResponseMeta.Usage.PromptTokens {
					ret.ResponseMeta.Usage.PromptTokens = msg.ResponseMeta.Usage.PromptTokens
				}
				if msg.ResponseMeta.Usage.CompletionTokens > ret.ResponseMeta.Usage.CompletionTokens {
					ret.ResponseMeta.Usage.CompletionTokens = msg.ResponseMeta.Usage.CompletionTokens
				}

				if msg.ResponseMeta.Usage.TotalTokens > ret.ResponseMeta.Usage.TotalTokens {
					ret.ResponseMeta.Usage.TotalTokens = msg.ResponseMeta.Usage.TotalTokens
				}

				if msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens > ret.ResponseMeta.Usage.PromptTokenDetails.CachedTokens {
					ret.ResponseMeta.Usage.PromptTokenDetails.CachedTokens = msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens
				}
			}

			if msg.ResponseMeta.LogProbs != nil {
				if ret.ResponseMeta.LogProbs == nil {
					ret.ResponseMeta.LogProbs = &LogProbs{}
				}

				ret.ResponseMeta.LogProbs.Content = append(ret.ResponseMeta.LogProbs.Content, msg.ResponseMeta.LogProbs.Content...)
			}

		}
	}

	if len(contents) > 0 {
		var sb strings.Builder
		sb.Grow(contentLen)
		for _, content := range contents {
			_, err := sb.WriteString(content)
			if err != nil {
				return nil, err
			}
		}

		ret.Content = sb.String()
	}
	if len(reasoningContents) > 0 {
		var sb strings.Builder
		sb.Grow(reasoningContentLen)
		for _, rc := range reasoningContents {
			_, err := sb.WriteString(rc)
			if err != nil {
				return nil, err
			}
		}

		ret.ReasoningContent = sb.String()
	}

	if len(toolCalls) > 0 {
		merged, err := concatToolCalls(toolCalls)
		if err != nil {
			return nil, err
		}

		ret.ToolCalls = merged
	}

	if len(extraList) > 0 {
		extra, err := concatExtra(extraList)
		if err != nil {
			return nil, fmt.Errorf("failed to concat message's extra: %w", err)
		}

		if len(extra) > 0 {
			ret.Extra = extra
		}
	}

	if len(multiContentParts) > 0 {
		ret.MultiContent = multiContentParts
	}

	if len(assistantGenMultiContentParts) > 0 {
		merged, err := concatAssistantMultiContent(assistantGenMultiContentParts)
		if err != nil {
			return nil, fmt.Errorf("failed to concat message's assistant multi content: %w", err)
		}
		ret.AssistantGenMultiContent = merged
	}

	return &ret, nil
}

// ConcatMessageStream drains a stream of messages and returns a single
// concatenated message representing the merged content.
func ConcatMessageStream(s *StreamReader[*Message]) (*Message, error) {
	defer s.Close()

	var msgs []*Message
	for {
		msg, err := s.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		msgs = append(msgs, msg)
	}

	return ConcatMessages(msgs)
}

// custom jinja env
var jinjaEnvOnce sync.Once
var jinjaEnv *gonja.Environment
var envInitErr error

const (
	jinjaInclude = "include"
	jinjaExtends = "extends"
	jinjaImport  = "import"
	jinjaFrom    = "from"
)

func getJinjaEnv() (*gonja.Environment, error) {
	jinjaEnvOnce.Do(func() {
		jinjaEnv = gonja.NewEnvironment(config.DefaultConfig, gonja.DefaultLoader)
		formatInitError := "init jinja env fail: %w"
		var err error
		if jinjaEnv.Statements.Exists(jinjaInclude) {
			err = jinjaEnv.Statements.Replace(jinjaInclude, func(parser *parser.Parser, args *parser.Parser) (nodes.Statement, error) {
				return nil, fmt.Errorf("keyword[include] has been disabled")
			})
			if err != nil {
				envInitErr = fmt.Errorf(formatInitError, err)
				return
			}
		}
		if jinjaEnv.Statements.Exists(jinjaExtends) {
			err = jinjaEnv.Statements.Replace(jinjaExtends, func(parser *parser.Parser, args *parser.Parser) (nodes.Statement, error) {
				return nil, fmt.Errorf("keyword[extends] has been disabled")
			})
			if err != nil {
				envInitErr = fmt.Errorf(formatInitError, err)
				return
			}
		}
		if jinjaEnv.Statements.Exists(jinjaFrom) {
			err = jinjaEnv.Statements.Replace(jinjaFrom, func(parser *parser.Parser, args *parser.Parser) (nodes.Statement, error) {
				return nil, fmt.Errorf("keyword[from] has been disabled")
			})
			if err != nil {
				envInitErr = fmt.Errorf(formatInitError, err)
				return
			}
		}
		if jinjaEnv.Statements.Exists(jinjaImport) {
			err = jinjaEnv.Statements.Replace(jinjaImport, func(parser *parser.Parser, args *parser.Parser) (nodes.Statement, error) {
				return nil, fmt.Errorf("keyword[import] has been disabled")
			})
			if err != nil {
				envInitErr = fmt.Errorf(formatInitError, err)
				return
			}
		}
	})
	return jinjaEnv, envInitErr
}
