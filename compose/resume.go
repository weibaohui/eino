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

package compose

import (
	"context"

	"github.com/cloudwego/eino/internal/core"
)

// GetInterruptState provides a type-safe way to check for and retrieve the persisted state from a previous interruption.
// It is the primary function a component should use to understand its past state.
//
// It returns three values:
//   - wasInterrupted (bool): True if the node was part of a previous interruption, regardless of whether state was provided.
//   - state (T): The typed state object, if it was provided and matches type `T`.
//   - hasState (bool): True if state was provided during the original interrupt and successfully cast to type `T`.
func GetInterruptState[T any](ctx context.Context) (wasInterrupted bool, hasState bool, state T) {
	return core.GetInterruptState[T](ctx)
}

// GetResumeContext checks if the current component is the target of a resume operation
// and retrieves any data provided by the user for that resumption.
//
// This function is typically called *after* a component has already determined it is in a
// resumed state by calling GetInterruptState.
//
// It returns three values:
//   - isResumeFlow: A boolean that is true if the current component's address was explicitly targeted
//     by a call to Resume() or ResumeWithData().
//   - hasData: A boolean that is true if data was provided for this component (i.e., not nil).
//   - data: The typed data provided by the user.
//
// ### How to Use This Function: A Decision Framework
//
// The correct usage pattern depends on the application's desired resume strategy.
//
// #### Strategy 1: Implicit "Resume All"
// In some use cases, any resume operation implies that *all* interrupted points should proceed.
// For example, if an application's UI only provides a single "Continue" button for a set of
// interruptions. In this model, a component can often just use `GetInterruptState` to see if
// `wasInterrupted` is true and then proceed with its logic, as it can assume it is an intended target.
// It may still call `GetResumeContext` to check for optional data, but the `isResumeFlow` flag is less critical.
//
// #### Strategy 2: Explicit "Targeted Resume" (Most Common)
// For applications with multiple, distinct interrupt points that must be resumed independently, it is
// crucial to differentiate which point is being resumed. This is the primary use case for the `isResumeFlow` flag.
//   - If `isResumeFlow` is `true`: Your component is the explicit target. You should consume
//     the `data` (if any) and complete your work.
//   - If `isResumeFlow` is `false`: Another component is the target. You MUST re-interrupt
//     (e.g., by returning `StatefulInterrupt(...)`) to preserve your state and allow the
//     resume signal to propagate.
//
// ### Guidance for Composite Components
//
// Composite components (like `Graph` or other `Runnable`s that contain sub-processes) have a dual role:
//  1. Check for Self-Targeting: A composite component can itself be the target of a resume
//     operation, for instance, to modify its internal state. It may call `GetResumeContext`
//     to check for data targeted at its own address.
//  2. Act as a Conduit: After checking for itself, its primary role is to re-execute its children,
//     allowing the resume context to flow down to them. It must not consume a resume signal
//     intended for one of its descendants.
func GetResumeContext[T any](ctx context.Context) (isResumeFlow bool, hasData bool, data T) {
	return core.GetResumeContext[T](ctx)
}

// GetCurrentAddress returns the hierarchical address of the currently executing component.
// The address is a sequence of segments, each identifying a structural part of the execution
// like an agent, a graph node, or a tool call. This can be useful for logging or debugging.
func GetCurrentAddress(ctx context.Context) Address {
	return core.GetCurrentAddress(ctx)
}

// Resume prepares a context for an "Explicit Targeted Resume" operation by targeting one or more
// components without providing data. It is a convenience wrapper around BatchResumeWithData.
//
// This is useful when the act of resuming is itself the signal, and no extra data is needed.
// The components at the provided addresses (interrupt IDs) will receive `isResumeFlow = true`
// when they call `GetResumeContext`.
func Resume(ctx context.Context, interruptIDs ...string) context.Context {
	resumeData := make(map[string]any, len(interruptIDs))
	for _, addr := range interruptIDs {
		resumeData[addr] = nil
	}
	return BatchResumeWithData(ctx, resumeData)
}

// ResumeWithData prepares a context to resume a single, specific component with data.
// It is the primary function for the "Explicit Targeted Resume" strategy when data is required.
// It is a convenience wrapper around BatchResumeWithData.
// The `interruptID` parameter is the unique interrupt ID of the target component.
//
// ResumeWithData 准备一个上下文，以便使用数据恢复单个特定组件。
// 当需要数据时，它是“显式定向恢复”策略的主要函数。
// 它是 BatchResumeWithData 的便捷包装器。
// `interruptID` 参数是目标组件的唯一中断 ID。
func ResumeWithData(ctx context.Context, interruptID string, data any) context.Context {
	return BatchResumeWithData(ctx, map[string]any{interruptID: data})
}

// BatchResumeWithData is the core function for preparing a resume context. It injects a map
// of resume targets and their corresponding data into the context.
//
// The `resumeData` map should contain the interrupt IDs (which are the string form of addresses) of the
// components to be resumed as keys. The value can be the resume data for that component, or `nil`
// if no data is needed (equivalent to using `Resume`).
//
// This function is the foundation for the "Explicit Targeted Resume" strategy. Components whose interrupt IDs
// are present as keys in the map will receive `isResumeFlow = true` when they call `GetResumeContext`.
//
// BatchResumeWithData 是准备恢复上下文的核心函数。它将恢复目标及其相应数据的映射注入上下文中。
//
// `resumeData` 映射应包含要恢复的组件的中断 ID（地址的字符串形式）作为键。
// 值可以是该组件的恢复数据，如果不需要数据（相当于使用 `Resume`），则可以为 `nil`。
//
// 此函数是“显式定向恢复”策略的基础。
// 中断 ID 作为键存在于映射中的组件在调用 `GetResumeContext` 时将收到 `isResumeFlow = true`。
func BatchResumeWithData(ctx context.Context, resumeData map[string]any) context.Context {
	return core.BatchResumeWithData(ctx, resumeData)
}

func getNodePath(ctx context.Context) (*NodePath, bool) {
	currentAddress := GetCurrentAddress(ctx)
	if len(currentAddress) == 0 {
		return nil, false
	}

	nodePath := make([]string, 0, len(currentAddress))
	for _, p := range currentAddress {
		if p.Type == AddressSegmentRunnable {
			nodePath = []string{}
			continue
		}

		nodePath = append(nodePath, p.ID)
	}

	return NewNodePath(nodePath...), len(nodePath) > 0
}

// AppendAddressSegment creates a new execution context for a sub-component (e.g., a graph node or a tool call).
//
// It extends the current context's address with a new segment and populates the new context with the
// appropriate interrupt state and resume data for that specific sub-address.
//
//   - ctx: The parent context, typically the one passed into the component's Invoke/Stream method.
//   - segType: The type of the new address segment (e.g., "node", "tool").
//   - segID: The unique ID for the new address segment.
//
// AppendAddressSegment 为子组件（例如，图节点或工具调用）创建一个新的执行上下文。
//
// 它使用新的片段扩展当前上下文的地址，并为该特定子地址填充适当的中断状态和恢复数据的新上下文。
//
//   - ctx: 父上下文，通常是传入组件的 Invoke/Stream 方法的上下文。
//   - segType: 新地址片段的类型（例如，“node”，“tool”）。
//   - segID: 新地址片段的唯一 ID。
func AppendAddressSegment(ctx context.Context, segType AddressSegmentType, segID string) context.Context {
	return core.AppendAddressSegment(ctx, segType, segID, "")
}

func appendToolAddressSegment(ctx context.Context, segID string, subID string) context.Context {
	return core.AppendAddressSegment(ctx, AddressSegmentTool, segID, subID)
}
