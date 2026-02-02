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

// Package planexecute implements a plan–execute–replan style agent.
package planexecute

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/internal/safe"
	"github.com/cloudwego/eino/schema"
)

func init() {
	schema.RegisterName[*defaultPlan]("_eino_adk_plan_execute_default_plan")
	schema.RegisterName[ExecutedStep]("_eino_adk_plan_execute_executed_step")
	schema.RegisterName[[]ExecutedStep]("_eino_adk_plan_execute_executed_steps")
}

// Plan 代表一个包含一系列可执行步骤的执行计划。
// 它支持 JSON 序列化和反序列化，同时提供访问第一个步骤的方法。
// 该接口使得 Agent 能够理解和操作结构化的任务列表。
type Plan interface {
	// FirstStep 返回计划中第一个待执行的步骤。
	FirstStep() string

	// Marshaler 将 Plan 序列化为 JSON。
	// 生成的 JSON 可以用于 Prompt 模版中，让 LLM 理解当前计划状态。
	json.Marshaler
	// Unmarshaler 将 JSON 内容反序列化为 Plan。
	// 用于解析结构化 Chat Model 的输出或工具调用的结果，将其转换为 Plan 结构。
	json.Unmarshaler
}

// NewPlan 是一个创建新 Plan 实例的函数类型。
type NewPlan func(ctx context.Context) Plan

// defaultPlan 是 Plan 接口的默认实现，只是一个简单的字符串切片。
//
// JSON Schema:
//
//	{
//	  "type": "object",
//	  "properties": {
//	    "steps": {
//	      "type": "array",
//	      "items": {
//	        "type": "string"
//	      },
//	      "description": "Ordered list of actions to be taken. Each step should be clear, actionable, and arranged in a logical sequence."
//	    }
//	  },
//	  "required": ["steps"]
//	}
type defaultPlan struct {
	// Steps 包含按顺序排列的待执行动作列表。
	// 每个步骤都应清晰、可操作，并按逻辑顺序排列。
	Steps []string `json:"steps"`
}

// FirstStep 返回计划中的第一步，如果不存在则返回空字符串。
func (p *defaultPlan) FirstStep() string {
	if len(p.Steps) == 0 {
		return ""
	}
	return p.Steps[0]
}

func (p *defaultPlan) MarshalJSON() ([]byte, error) {
	type planTyp defaultPlan
	return sonic.Marshal((*planTyp)(p))
}

func (p *defaultPlan) UnmarshalJSON(bytes []byte) error {
	type planTyp defaultPlan
	return sonic.Unmarshal(bytes, (*planTyp)(p))
}

// Response 代表对用户的最终响应。
// 该结构体用于模型生成的最终响应的 JSON 序列化/反序列化。
type Response struct {
	// Response 是提供给用户的完整响应内容。
	// 该字段是必需的。
	Response string `json:"response"`
}

var (
	// PlanToolInfo 定义了可与 ToolCallingChatModel 一起使用的 Plan 工具的 schema。
	// 该 schema 指示模型生成带有有序步骤的结构化计划。
	PlanToolInfo = schema.ToolInfo{
		Name: "plan",
		Desc: "Plan with a list of steps to execute in order. Each step should be clear, actionable, and arranged in a logical sequence. The output will be used to guide the execution process.",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"steps": {
					Type:     schema.Array,
					ElemInfo: &schema.ParameterInfo{Type: schema.String},
					Desc:     "different steps to follow, should be in sorted order",
					Required: true,
				},
			},
		),
	}

	// RespondToolInfo 定义了可与 ToolCallingChatModel 一起使用的 respond 工具的 schema。
	// 该 schema 指示模型生成对用户的直接响应。
	RespondToolInfo = schema.ToolInfo{
		Name: "respond",
		Desc: "Generate a direct response to the user. Use this tool when you have all the information needed to provide a final answer.",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"response": {
					Type:     schema.String,
					Desc:     "The complete response to provide to the user",
					Required: true,
				},
			},
		),
	}

	// PlannerPrompt 是 Planner 的提示词模版。
	// 它为 Planner 提供上下文和指导，说明如何生成 Plan。
	PlannerPrompt = prompt.FromMessages(schema.FString,
		// 规划器提示词的中文翻译：
		// 你是一个专业的规划智能体。给定一个目标，请创建一个全面的分步骤计划以实现该目标。
		//
		// ## 你的任务
		// 分析目标并生成一个战略计划，将目标分解为可管理、可执行的步骤。
		//
		// ## 规划要求
		// 计划中的每一步必须：
		// - **具体且可操作**：清晰的指令，可以毫无歧义地执行
		// - **自包含**：包含所有必要的上下文、参数和要求
		// - **可独立执行**：可以由智能体执行，而不依赖于其他步骤
		// - **逻辑排序**：以最佳顺序排列，以便高效执行
		// - **聚焦目标**：直接有助于实现主要目标
		//
		// ## 规划指南
		// - 消除冗余或不必要的步骤
		// - 包含每一步的相关约束、参数和成功标准
		// - 确保最后一步产生完整的答案或可交付成果
		// - 预见潜在挑战并包含缓解策略
		// - 逻辑地构建步骤，使其相互建立
		// - 提供足够的细节以确保成功执行
		//
		// ## 质量标准
		// - 计划完整性：是否涵盖了目标的所有方面？
		// - 步骤清晰度：每个步骤是否可以独立理解和执行？
		// - 逻辑流程：步骤是否遵循合理的进展？
		// - 效率：这是实现目标的最直接路径吗？
		// - 适应性：计划能否处理意外结果或变化？

		schema.SystemMessage(`You are an expert planning agent. Given an objective, create a comprehensive step-by-step plan to achieve the objective.

## YOUR TASK
Analyze the objective and generate a strategic plan that breaks down the goal into manageable, executable steps.

## PLANNING REQUIREMENTS
Each step in your plan must be:
- **Specific and actionable**: Clear instructions that can be executed without ambiguity
- **Self-contained**: Include all necessary context, parameters, and requirements
- **Independently executable**: Can be performed by an agent without dependencies on other steps
- **Logically sequenced**: Arranged in optimal order for efficient execution
- **Objective-focused**: Directly contribute to achieving the main goal

## PLANNING GUIDELINES
- Eliminate redundant or unnecessary steps
- Include relevant constraints, parameters, and success criteria for each step
- Ensure the final step produces a complete answer or deliverable
- Anticipate potential challenges and include mitigation strategies
- Structure steps to build upon each other logically
- Provide sufficient detail for successful execution

## QUALITY CRITERIA
- Plan completeness: Does it address all aspects of the objective?
- Step clarity: Can each step be understood and executed independently?
- Logical flow: Do steps follow a sensible progression?
- Efficiency: Is this the most direct path to the objective?
- Adaptability: Can the plan handle unexpected results or changes?`),
		schema.MessagesPlaceholder("input", false),
	)

	// ExecutorPrompt 是 Executor 的提示词模版。
	// 它为 Executor 提供上下文和指导，说明如何执行 Task。
	ExecutorPrompt = prompt.FromMessages(schema.FString,
		// 执行器提示词的中文翻译：
		// 你是一个勤奋而 meticulous 的执行智能体。按照给定的计划，仔细和彻底地执行你的任务。
		schema.SystemMessage(`You are a diligent and meticulous executor agent. Follow the given plan and execute your tasks carefully and thoroughly.`),

		// ## 你的任务
		// 按照给定的计划，仔细和彻底地执行你的任务。
		schema.UserMessage(`## OBJECTIVE
{input}
## Given the following plan:
{plan}
## COMPLETED STEPS & RESULTS
{executed_steps}
## Your task is to execute the first step, which is: 
{step}`))

	// ReplannerPrompt 是 Replanner 的提示词模版。
	// 它为 Replanner 提供上下文和指导，说明如何重新生成 Plan。
	ReplannerPrompt = prompt.FromMessages(schema.FString,
		// 重新规划器提示词的中文翻译：
		// 你是一个专业的重新规划智能体。根据当前进度，分析当前状态并确定最优的下一个操作。
		//
		// ## 你的任务
		// 根据上述进展，你必须选择恰好一个操作：
		//
		// ### 选项 1：完成（如果目标已完全实现）
		// 调用 '{respond_tool}'，包含：
		// - 全面的最终答案
		// - 清晰总结目标是如何实现的结论
		// - 执行过程中的关键洞察
		//
		// ### 选项 2：继续（如果需要更多工作）
		// 调用 '{plan_tool}'，提供一个修订后的计划，该计划：
		// - 仅包含剩余步骤（排除已完成的步骤）
		// - 结合从已执行步骤中学到的经验
		// - 解决发现的任何差距或问题
		// - 保持逻辑步骤顺序
		//
		// ## 规划要求
		// 计划中的每一步必须：
		// - **具体且可操作**：可以毫无歧义地执行的清晰指令
		// - **自包含**：包含所有必要的上下文、参数和要求
		// - **可独立执行**：可以由智能体执行，而不依赖于其他步骤
		// - **逻辑排序**：以最佳顺序排列，以便高效执行
		// - **聚焦目标**：直接有助于实现主要目标
		//
		// ## 规划指南
		// - 消除冗余或不必要的步骤
		// - 根据新信息调整策略
		// - 包含每一步的相关约束、参数和成功标准
		//
		// ## 决策标准
		// - 原始目标是否已完全满足？
		// - 是否还有剩余要求或子目标？
		// - 结果是否表明需要调整策略？
		// - 还需要哪些具体行动？
		schema.SystemMessage(
			`You are going to review the progress toward an objective. Analyze the current state and determine the optimal next action.

## YOUR TASK
Based on the progress above, you MUST choose exactly ONE action:

### Option 1: COMPLETE (if objective is fully achieved)
Call '{respond_tool}' with:
- A comprehensive final answer
- Clear conclusion summarizing how the objective was met
- Key insights from the execution process

### Option 2: CONTINUE (if more work is needed)
Call '{plan_tool}' with a revised plan that:
- Contains ONLY remaining steps (exclude completed ones)
- Incorporates lessons learned from executed steps
- Addresses any gaps or issues discovered
- Maintains logical step sequence

## PLANNING REQUIREMENTS
Each step in your plan must be:
- **Specific and actionable**: Clear instructions that can be executed without ambiguity
- **Self-contained**: Include all necessary context, parameters, and requirements
- **Independently executable**: Can be performed by an agent without dependencies on other steps
- **Logically sequenced**: Arranged in optimal order for efficient execution
- **Objective-focused**: Directly contribute to achieving the main goal

## PLANNING GUIDELINES
- Eliminate redundant or unnecessary steps
- Adapt strategy based on new information
- Include relevant constraints, parameters, and success criteria for each step

## DECISION CRITERIA
- Has the original objective been completely satisfied?
- Are there any remaining requirements or sub-goals?
- Do the results suggest a need for strategy adjustment?
- What specific actions are still required?`),
		schema.UserMessage(`## OBJECTIVE
{input}

## ORIGINAL PLAN
{plan}

## COMPLETED STEPS & RESULTS
{executed_steps}`),
	)
)

const (
	// UserInputSessionKey 是会话中存储用户输入的键。
	UserInputSessionKey = "UserInput"

	// PlanSessionKey 是会话中存储当前计划的键。
	PlanSessionKey = "Plan"

	// ExecutedStepSessionKey 是会话中存储执行结果的键。
	ExecutedStepSessionKey = "ExecutedStep"

	// ExecutedStepsSessionKey 是会话中存储所有已执行步骤结果的键。
	ExecutedStepsSessionKey = "ExecutedSteps"
)

// PlannerConfig 提供了创建 Planner Agent 的配置选项。
// 有两种方式配置 Planner 以生成结构化的 Plan 输出：
//  1. 使用 ChatModelWithFormattedOutput：预配置为输出 Plan 格式的模型
//  2. 使用 ToolCallingChatModel + ToolInfo：使用工具调用来生成 Plan 结构的模型
type PlannerConfig struct {
	// ChatModelWithFormattedOutput 是一个预配置为输出 Plan 格式的模型。
	// 通过配置模型直接输出结构化数据来创建此模型。
	// 示例参考：https://github.com/cloudwego/eino-ext/blob/main/components/model/openai/examples/structured/structured.go
	ChatModelWithFormattedOutput model.BaseChatModel

	// ToolCallingChatModel 是一个支持工具调用能力的模型。
	// 当提供 ToolInfo 时，它将使用工具调用来生成 Plan 结构。
	ToolCallingChatModel model.ToolCallingChatModel

	// ToolInfo 定义了使用工具调用时 Plan 结构的 schema。
	// 可选。如果未提供，将使用默认的 PlanToolInfo。
	ToolInfo *schema.ToolInfo

	// GenInputFn 是生成 Planner 输入消息的函数。
	// 可选。如果未提供，将使用 defaultGenPlannerInputFn。
	GenInputFn GenPlannerModelInputFn

	// NewPlan 为 JSON 创建一个新的 Plan 实例。
	// 返回的 Plan 将用于反序列化模型生成的 JSON 输出。
	// 可选。如果未提供，将使用 defaultNewPlan。
	NewPlan NewPlan
}

// GenPlannerModelInputFn 是一个生成 Planner 输入消息的函数类型。
type GenPlannerModelInputFn func(ctx context.Context, userInput []adk.Message) ([]adk.Message, error)

// defaultNewPlan 创建默认的 defaultPlan 实例。
func defaultNewPlan(ctx context.Context) Plan {
	return &defaultPlan{}
}

// defaultGenPlannerInputFn 使用默认的 PlannerPrompt 生成输入消息。
func defaultGenPlannerInputFn(ctx context.Context, userInput []adk.Message) ([]adk.Message, error) {
	msgs, err := PlannerPrompt.Format(ctx, map[string]any{
		"input": userInput,
	})
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

// planner 实现了负责生成执行计划的 Agent。
type planner struct {
	toolCall   bool
	chatModel  model.BaseChatModel
	genInputFn GenPlannerModelInputFn
	newPlan    NewPlan
}

// Name 返回 planner Agent 的名称。
func (p *planner) Name(_ context.Context) string {
	return "planner"
}

// Description 返回 planner Agent 的描述。
func (p *planner) Description(_ context.Context) string {
	return "a planner agent"
}

// argToContent 将工具调用的参数转换为 Assistant 消息的内容。
// 这用于在流式传输时将工具调用参数展示给用户（如果需要）。
func argToContent(msg adk.Message) (adk.Message, error) {
	if len(msg.ToolCalls) == 0 {
		return nil, schema.ErrNoValue
	}

	return schema.AssistantMessage(msg.ToolCalls[0].Function.Arguments, nil), nil
}

// Run 执行 planner Agent 的主逻辑。
// 1. 生成输入消息。
// 2. 调用 ChatModel 生成计划（可能是直接文本或工具调用）。
// 3. 解析生成的计划并存储到 Session 中。
func (p *planner) Run(ctx context.Context, input *adk.AgentInput,
	_ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {

	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	adk.AddSessionValue(ctx, UserInputSessionKey, input.Messages)

	go func() {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				e := safe.NewPanicErr(panicErr, debug.Stack())
				generator.Send(&adk.AgentEvent{Err: e})
			}

			generator.Close()
		}()

		c := compose.NewChain[*adk.AgentInput, Plan]().
			AppendLambda(
				compose.InvokableLambda(func(ctx context.Context, input *adk.AgentInput) (output []adk.Message, err error) {
					return p.genInputFn(ctx, input.Messages)
				}),
			).
			AppendChatModel(p.chatModel).
			AppendLambda(
				compose.CollectableLambda(func(ctx context.Context, sr *schema.StreamReader[adk.Message]) (adk.Message, error) {
					if input.EnableStreaming {
						ss := sr.Copy(2)
						var sOutput *schema.StreamReader[*schema.Message]
						if p.toolCall {
							sOutput = schema.StreamReaderWithConvert(ss[0], argToContent)
						} else {
							sOutput = ss[0]
						}

						generator.Send(adk.EventFromMessage(nil, sOutput, schema.Assistant, ""))

						return schema.ConcatMessageStream(ss[1])
					}

					msg, err := schema.ConcatMessageStream(sr)
					if err != nil {
						return nil, err
					}

					var output adk.Message
					if p.toolCall {
						if len(msg.ToolCalls) == 0 {
							return nil, fmt.Errorf("no tool call")
						}
						output = schema.AssistantMessage(msg.ToolCalls[0].Function.Arguments, nil)
					} else {
						output = msg
					}

					generator.Send(adk.EventFromMessage(output, nil, schema.Assistant, ""))

					return msg, nil
				}),
			).
			AppendLambda(
				compose.InvokableLambda(func(ctx context.Context, msg adk.Message) (plan Plan, err error) {
					var planJSON string
					if p.toolCall {
						if len(msg.ToolCalls) == 0 {
							return nil, fmt.Errorf("no tool call")
						}
						planJSON = msg.ToolCalls[0].Function.Arguments
					} else {
						planJSON = msg.Content
					}

					plan = p.newPlan(ctx)
					err = plan.UnmarshalJSON([]byte(planJSON))
					if err != nil {
						return nil, fmt.Errorf("unmarshal plan error: %w", err)
					}

					adk.AddSessionValue(ctx, PlanSessionKey, plan)

					return plan, nil
				}),
			)

		var opts []compose.Option
		if p.toolCall {
			opts = append(opts, compose.WithChatModelOption(model.WithToolChoice(schema.ToolChoiceForced)))
		}

		r, err := c.Compile(ctx, compose.WithGraphName(p.Name(ctx)))
		if err != nil { // unexpected
			generator.Send(&adk.AgentEvent{Err: err})
			return
		}

		_, err = r.Stream(ctx, input, opts...)
		if err != nil {
			generator.Send(&adk.AgentEvent{Err: err})
			return
		}
	}()

	return iterator
}

// NewPlanner 基于提供的配置创建一个新的 Planner Agent。
// Planner Agent 使用 ChatModelWithFormattedOutput 或 ToolCallingChatModel+ToolInfo
// 来生成结构化的 Plan 输出。
//
// 如果提供了 ChatModelWithFormattedOutput，将直接使用它。
// 如果提供了 ToolCallingChatModel，它将配置 ToolInfo（默认为 PlanToolInfo）
// 以生成结构化的 Plan 输出。
func NewPlanner(_ context.Context, cfg *PlannerConfig) (adk.Agent, error) {
	var chatModel model.BaseChatModel
	var toolCall bool
	if cfg.ChatModelWithFormattedOutput != nil {
		chatModel = cfg.ChatModelWithFormattedOutput
	} else {
		toolCall = true
		toolInfo := cfg.ToolInfo
		if toolInfo == nil {
			toolInfo = &PlanToolInfo
		}

		var err error
		chatModel, err = cfg.ToolCallingChatModel.WithTools([]*schema.ToolInfo{toolInfo})
		if err != nil {
			return nil, err
		}
	}

	inputFn := cfg.GenInputFn
	if inputFn == nil {
		inputFn = defaultGenPlannerInputFn
	}

	planParser := cfg.NewPlan
	if planParser == nil {
		planParser = defaultNewPlan
	}

	return &planner{
		toolCall:   toolCall,
		chatModel:  chatModel,
		genInputFn: inputFn,
		newPlan:    planParser,
	}, nil
}

// ExecutionContext 是 Executor 和 Planner 的输入信息。
type ExecutionContext struct {
	UserInput     []adk.Message
	Plan          Plan
	ExecutedSteps []ExecutedStep
}

// GenModelInputFn 是一个生成 Executor 和 Planner 输入消息的函数类型。
type GenModelInputFn func(ctx context.Context, in *ExecutionContext) ([]adk.Message, error)

// ExecutorConfig 提供了创建 Executor Agent 的配置选项。
type ExecutorConfig struct {
	// Model 是 Executor 使用的 Chat Model。
	Model model.ToolCallingChatModel

	// ToolsConfig 指定了 Executor 可用的工具。
	ToolsConfig adk.ToolsConfig

	// MaxIterations 定义了 ChatModel 生成周期的上限。
	// 如果超过此限制，Agent 将以错误终止。
	// 可选。默认为 20。
	MaxIterations int

	// GenInputFn 生成 Executor 的输入消息。
	// 可选。如果未提供，将使用 defaultGenExecutorInputFn。
	GenInputFn GenModelInputFn
}

// ExecutedStep 记录了一个步骤的执行结果。
type ExecutedStep struct {
	Step   string
	Result string
}

// NewExecutor 创建一个新的 Executor Agent。
func NewExecutor(ctx context.Context, cfg *ExecutorConfig) (adk.Agent, error) {

	genInputFn := cfg.GenInputFn
	if genInputFn == nil {
		genInputFn = defaultGenExecutorInputFn
	}
	genInput := func(ctx context.Context, instruction string, _ *adk.AgentInput) ([]adk.Message, error) {

		plan, ok := adk.GetSessionValue(ctx, PlanSessionKey)
		if !ok {
			panic("impossible: plan not found")
		}
		plan_ := plan.(Plan)

		userInput, ok := adk.GetSessionValue(ctx, UserInputSessionKey)
		if !ok {
			panic("impossible: user input not found")
		}
		userInput_ := userInput.([]adk.Message)

		var executedSteps_ []ExecutedStep
		executedStep, ok := adk.GetSessionValue(ctx, ExecutedStepsSessionKey)
		if ok {
			executedSteps_ = executedStep.([]ExecutedStep)
		}

		in := &ExecutionContext{
			UserInput:     userInput_,
			Plan:          plan_,
			ExecutedSteps: executedSteps_,
		}

		msgs, err := genInputFn(ctx, in)
		if err != nil {
			return nil, err
		}

		return msgs, nil
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "executor",
		Description:   "an executor agent",
		Model:         cfg.Model,
		ToolsConfig:   cfg.ToolsConfig,
		GenModelInput: genInput,
		MaxIterations: cfg.MaxIterations,
		OutputKey:     ExecutedStepSessionKey,
	})
	if err != nil {
		return nil, err
	}

	return agent, nil
}

// defaultGenExecutorInputFn 使用默认的 ExecutorPrompt 生成输入消息。
// 它会将当前计划、已执行步骤和下一步骤组合成 Prompt。
func defaultGenExecutorInputFn(ctx context.Context, in *ExecutionContext) ([]adk.Message, error) {

	planContent, err := in.Plan.MarshalJSON()
	if err != nil {
		return nil, err
	}

	userMsgs, err := ExecutorPrompt.Format(ctx, map[string]any{
		"input":          formatInput(in.UserInput),
		"plan":           string(planContent),
		"executed_steps": formatExecutedSteps(in.ExecutedSteps),
		"step":           in.Plan.FirstStep(),
	})
	if err != nil {
		return nil, err
	}

	return userMsgs, nil
}

// replanner 实现了负责重新规划的 Agent。
// 它根据执行结果判断是继续执行、修改计划还是结束任务。
type replanner struct {
	chatModel   model.ToolCallingChatModel
	planTool    *schema.ToolInfo
	respondTool *schema.ToolInfo

	genInputFn GenModelInputFn
	newPlan    NewPlan
}

type ReplannerConfig struct {
	// ChatModel 是支持工具调用能力的模型。
	// 它将配置 PlanTool 和 RespondTool 以生成更新的计划或响应。
	ChatModel model.ToolCallingChatModel

	// PlanTool 定义了可与 ToolCallingChatModel 一起使用的 Plan 工具的 schema。
	// 可选。如果未提供，将使用默认的 PlanToolInfo。
	PlanTool *schema.ToolInfo

	// RespondTool 定义了可与 ToolCallingChatModel 一起使用的 response 工具的 schema。
	// 可选。如果未提供，将使用默认的 RespondToolInfo。
	RespondTool *schema.ToolInfo

	// GenInputFn 生成 Replanner 的输入消息。
	// 可选。如果未提供，将使用 buildGenReplannerInputFn。
	GenInputFn GenModelInputFn

	// NewPlan 创建一个新的 Plan 实例。
	// 返回的 Plan 将用于反序列化模型从 PlanTool 生成的 JSON 输出。
	// 可选。如果未提供，将使用 defaultNewPlan。
	NewPlan NewPlan
}

// formatInput 将输入消息列表格式化为字符串。
func formatInput(input []adk.Message) string {
	var sb strings.Builder
	for _, msg := range input {
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatExecutedSteps 将已执行步骤列表格式化为字符串，用于 Prompt 上下文。
func formatExecutedSteps(results []ExecutedStep) string {
	var sb strings.Builder
	for _, result := range results {
		sb.WriteString(fmt.Sprintf("Step: %s\nResult: %s\n\n", result.Step, result.Result))
	}

	return sb.String()
}

// Name 返回 replanner Agent 的名称。
func (r *replanner) Name(_ context.Context) string {
	return "replanner"
}

// Description 返回 replanner Agent 的描述。
func (r *replanner) Description(_ context.Context) string {
	return "a replanner agent"
}

// genInput 为 Replanner 生成输入消息。
// 它从 Session 中收集执行结果、当前计划和用户输入，构建上下文供模型决策。
func (r *replanner) genInput(ctx context.Context) ([]adk.Message, error) {

	executedStep, ok := adk.GetSessionValue(ctx, ExecutedStepSessionKey)
	if !ok {
		panic("impossible: execute result not found")
	}
	executedStep_ := executedStep.(string)

	plan, ok := adk.GetSessionValue(ctx, PlanSessionKey)
	if !ok {
		panic("impossible: plan not found")
	}
	plan_ := plan.(Plan)
	step := plan_.FirstStep()

	var executedSteps_ []ExecutedStep
	executedSteps, ok := adk.GetSessionValue(ctx, ExecutedStepsSessionKey)
	if ok {
		executedSteps_ = executedSteps.([]ExecutedStep)
	}

	executedSteps_ = append(executedSteps_, ExecutedStep{
		Step:   step,
		Result: executedStep_,
	})
	adk.AddSessionValue(ctx, ExecutedStepsSessionKey, executedSteps_)

	userInput, ok := adk.GetSessionValue(ctx, UserInputSessionKey)
	if !ok {
		panic("impossible: user input not found")
	}
	userInput_ := userInput.([]adk.Message)

	in := &ExecutionContext{
		UserInput:     userInput_,
		Plan:          plan_,
		ExecutedSteps: executedSteps_,
	}
	genInputFn := r.genInputFn
	if genInputFn == nil {
		genInputFn = buildGenReplannerInputFn(r.planTool.Name, r.respondTool.Name)
	}
	msgs, err := genInputFn(ctx, in)
	if err != nil {
		return nil, err
	}

	return msgs, nil
}

// Run 执行 replanner Agent 的主逻辑。
// 1. 收集执行上下文（计划、结果、用户输入）。
// 2. 调用 ChatModel 进行决策（使用 plan 工具或 respond 工具）。
// 3. 根据决策结果更新计划或结束循环。
func (r *replanner) Run(ctx context.Context, input *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				e := safe.NewPanicErr(panicErr, debug.Stack())
				generator.Send(&adk.AgentEvent{Err: e})
			}

			generator.Close()
		}()

		callOpt := model.WithToolChoice(schema.ToolChoiceForced)

		c := compose.NewChain[struct{}, any]().
			AppendLambda(
				compose.InvokableLambda(func(ctx context.Context, input struct{}) (output []adk.Message, err error) {
					return r.genInput(ctx)
				}),
			).
			AppendChatModel(r.chatModel).
			AppendLambda(
				compose.CollectableLambda(func(ctx context.Context, sr *schema.StreamReader[adk.Message]) (adk.Message, error) {
					if input.EnableStreaming {
						ss := sr.Copy(2)
						sOutput := schema.StreamReaderWithConvert(ss[0], argToContent)
						generator.Send(adk.EventFromMessage(nil, sOutput, schema.Assistant, ""))
						return schema.ConcatMessageStream(ss[1])
					}

					msg, err := schema.ConcatMessageStream(sr)
					if err != nil {
						return nil, err
					}
					if len(msg.ToolCalls) > 0 {
						output := schema.AssistantMessage(msg.ToolCalls[0].Function.Arguments, nil)
						generator.Send(adk.EventFromMessage(output, nil, schema.Assistant, ""))
					}
					return msg, nil
				}),
			).
			AppendLambda(
				compose.InvokableLambda(func(ctx context.Context, msg adk.Message) (msgOrPlan any, err error) {
					if len(msg.ToolCalls) == 0 {
						return nil, fmt.Errorf("no tool call")
					}

					// exit
					if msg.ToolCalls[0].Function.Name == r.respondTool.Name {
						action := adk.NewBreakLoopAction(r.Name(ctx))
						generator.Send(&adk.AgentEvent{Action: action})
						return msg, nil
					}

					// replan
					if msg.ToolCalls[0].Function.Name != r.planTool.Name {
						return nil, fmt.Errorf("unexpected tool call: %s", msg.ToolCalls[0].Function.Name)
					}

					plan := r.newPlan(ctx)
					if err = plan.UnmarshalJSON([]byte(msg.ToolCalls[0].Function.Arguments)); err != nil {
						return nil, fmt.Errorf("unmarshal plan error: %w", err)
					}

					adk.AddSessionValue(ctx, PlanSessionKey, plan)

					return plan, nil
				}),
			)

		runnable, err := c.Compile(ctx, compose.WithGraphName(r.Name(ctx)))
		if err != nil {
			generator.Send(&adk.AgentEvent{Err: err})
			return
		}

		_, err = runnable.Stream(ctx, struct{}{}, compose.WithChatModelOption(callOpt))
		if err != nil {
			generator.Send(&adk.AgentEvent{Err: err})
			return
		}
	}()

	return iterator
}

// buildGenReplannerInputFn 构建一个生成 Replanner 输入消息的函数。
// 它允许注入工具名称以适应不同的工具配置。
func buildGenReplannerInputFn(planToolName, respondToolName string) GenModelInputFn {
	return func(ctx context.Context, in *ExecutionContext) ([]adk.Message, error) {
		planContent, err := in.Plan.MarshalJSON()
		if err != nil {
			return nil, err
		}
		msgs, err := ReplannerPrompt.Format(ctx, map[string]any{
			"plan":           string(planContent),
			"input":          formatInput(in.UserInput),
			"executed_steps": formatExecutedSteps(in.ExecutedSteps),
			"plan_tool":      planToolName,
			"respond_tool":   respondToolName,
		})
		if err != nil {
			return nil, err
		}

		return msgs, nil
	}
}

// NewReplanner 创建一个 plan-execute-replan 代理，并连接 plan 和 respond 工具。
// 它配置提供的 ToolCallingChatModel 使用这些工具，并返回一个 Agent。
func NewReplanner(_ context.Context, cfg *ReplannerConfig) (adk.Agent, error) {
	planTool := cfg.PlanTool
	if planTool == nil {
		planTool = &PlanToolInfo
	}

	respondTool := cfg.RespondTool
	if respondTool == nil {
		respondTool = &RespondToolInfo
	}

	chatModel, err := cfg.ChatModel.WithTools([]*schema.ToolInfo{planTool, respondTool})
	if err != nil {
		return nil, err
	}

	planParser := cfg.NewPlan
	if planParser == nil {
		planParser = defaultNewPlan
	}

	return &replanner{
		chatModel:   chatModel,
		planTool:    planTool,
		respondTool: respondTool,
		genInputFn:  cfg.GenInputFn,
		newPlan:     planParser,
	}, nil
}

// Config 提供了创建 plan-execute-replan Agent 的配置选项。
type Config struct {
	// Planner 指定生成计划的 Agent。
	// 可以使用提供的 NewPlanner 创建 Planner Agent。
	Planner adk.Agent

	// Executor 指定执行 Planner 或 Replanner 生成的计划的 Agent。
	// 可以使用提供的 NewExecutor 创建 Executor Agent。
	Executor adk.Agent

	// Replanner 指定重新规划计划的 Agent。
	// 可以使用提供的 NewReplanner 创建 Replanner Agent。
	Replanner adk.Agent

	// MaxIterations 定义了 'execute-replan' 循环的最大次数。
	// 可选。如果未提供，默认值为 10。
	MaxIterations int
}

// New 使用给定的配置创建一个新的 plan-execute-replan Agent。
// plan-execute-replan 模式分三个阶段工作：
// 1. Planning（规划）：生成一个包含清晰、可操作步骤的结构化计划
// 2. Execution（执行）：执行计划的第一步
// 3. Replanning（重规划）：评估进度并完成任务或修改计划
// 这种方法通过迭代改进来实现复杂问题的解决。
func New(ctx context.Context, cfg *Config) (adk.ResumableAgent, error) {
	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}
	loop, err := adk.NewLoopAgent(ctx, &adk.LoopAgentConfig{
		Name:          "execute_replan",
		SubAgents:     []adk.Agent{cfg.Executor, cfg.Replanner},
		MaxIterations: maxIterations,
	})
	if err != nil {
		return nil, err
	}

	return adk.NewSequentialAgent(ctx, &adk.SequentialAgentConfig{
		Name:      "plan_execute_replan",
		SubAgents: []adk.Agent{cfg.Planner, loop},
	})
}
