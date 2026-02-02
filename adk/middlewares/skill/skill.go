/*
 * Copyright 2025 CloudWeGo Authors
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

// Package skill provides the skill middleware, types, and a local filesystem backend.
package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/slongfield/pyfmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// FrontMatter defines the metadata of a skill.
// FrontMatter 定义 Skill 的元数据。
type FrontMatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Skill represents a loaded skill.
// Skill 代表一个已加载的 Skill。
type Skill struct {
	FrontMatter
	Content       string
	BaseDirectory string
}

// Backend defines the interface for retrieving skills.
// Backend 定义了获取 Skill 的接口。
type Backend interface {
	List(ctx context.Context) ([]FrontMatter, error)
	Get(ctx context.Context, name string) (Skill, error)
}

// Config is the configuration for the skill middleware.
// Config 是 Skill 中间件的配置。
type Config struct {
	// Backend is the backend for retrieving skills.
	// Backend 是用于检索 Skill 的后端。
	Backend Backend
	// SkillToolName is the custom name for the skill tool. If nil, the default name "skill" is used.
	// SkillToolName 是 Skill 工具的自定义名称。如果为 nil，则使用默认名称 "skill"。
	SkillToolName *string
	// UseChinese controls whether to use Chinese prompts. When set to true, Chinese prompts are used;
	// when set to false (default), English prompts are used.
	// UseChinese 控制是否使用中文提示词。当设置为 true 时，使用中文提示词；
	// 当设置为 false（默认）时，使用英文提示词。
	UseChinese bool
}

// New creates a new skill middleware.
// It provides a tool for the agent to use skills.
// New 创建一个新的 Skill 中间件。
// 它为 agent 提供了使用 Skill 的工具。
func New(ctx context.Context, config *Config) (adk.AgentMiddleware, error) {
	if config == nil {
		return adk.AgentMiddleware{}, fmt.Errorf("config is required")
	}
	if config.Backend == nil {
		return adk.AgentMiddleware{}, fmt.Errorf("backend is required")
	}

	name := toolName
	if config.SkillToolName != nil {
		name = *config.SkillToolName
	}

	return adk.AgentMiddleware{
		AdditionalInstruction: buildSystemPrompt(name, config.UseChinese),
		AdditionalTools:       []tool.BaseTool{&skillTool{b: config.Backend, toolName: name, useChinese: config.UseChinese}},
	}, nil
}

func buildSystemPrompt(skillToolName string, useChinese bool) string {
	prompt := systemPrompt
	if useChinese {
		prompt = systemPromptChinese
	}
	return pyfmt.Must(prompt, map[string]string{
		"tool_name": skillToolName,
	})
}

type skillTool struct {
	b          Backend
	toolName   string
	useChinese bool
}

type descriptionTemplateHelper struct {
	Matters []FrontMatter
}

func (s *skillTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	skills, err := s.b.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list skills: %w", err)
	}

	desc, err := renderToolDescription(skills)
	if err != nil {
		return nil, fmt.Errorf("failed to render skill tool description: %w", err)
	}

	descBase := toolDescriptionBase
	paramDesc := "The skill name (no arguments). E.g., \"pdf\" or \"xlsx\""
	if s.useChinese {
		descBase = toolDescriptionBaseChinese
		paramDesc = "技能名称（无需其他参数）。例如：\"pdf\" 或 \"xlsx\""
	}

	return &schema.ToolInfo{
		Name: s.toolName,
		Desc: descBase + desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"skill": {
				Type:     schema.String,
				Desc:     paramDesc,
				Required: true,
			},
		}),
	}, nil
}

type inputArguments struct {
	Skill string `json:"skill"`
}

func (s *skillTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	args := &inputArguments{}
	err := json.Unmarshal([]byte(argumentsInJSON), args)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal arguments: %w", err)
	}
	skill, err := s.b.Get(ctx, args.Skill)
	if err != nil {
		return "", fmt.Errorf("failed to get skill: %w", err)
	}

	resultFmt := toolResult
	contentFmt := userContent
	if s.useChinese {
		resultFmt = toolResultChinese
		contentFmt = userContentChinese
	}

	return fmt.Sprintf(resultFmt, skill.Name) + fmt.Sprintf(contentFmt, skill.BaseDirectory, skill.Content), nil
}

func renderToolDescription(matters []FrontMatter) (string, error) {
	tpl, err := template.New("skills").Parse(toolDescriptionTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tpl.Execute(&buf, descriptionTemplateHelper{Matters: matters})
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
