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

package core

import "context"

// GetInterruptState provides a type-safe way to check for and retrieve the persisted state from a previous interruption.
// It is the primary function a component should use to understand its past state.
//
// GetInterruptState 提供了一种类型安全的方式来检查和检索先前中断的持久化状态。
// 它是组件用来了解其过去状态的主要函数。
//
// It returns three values:
//   - wasInterrupted (bool): True if the node was part of a previous interruption, regardless of whether state was provided.
//   - state (T): The typed state object, if it was provided and matches type `T`.
//   - hasState (bool): True if state was provided during the original interrupt and successfully cast to type `T`.
//
// 它返回三个值：
//   - wasInterrupted (bool): 如果节点是先前中断的一部分，则为 true，无论是否提供了状态。
//   - state (T): 类型化的状态对象，如果已提供且匹配类型 `T`。
//   - hasState (bool): 如果在原始中断期间提供了状态并成功转换为类型 `T`，则为 true。
func GetInterruptState[T any](ctx context.Context) (wasInterrupted bool, hasState bool, state T) {
	rCtx, ok := getRunCtx(ctx)
	if !ok || rCtx.interruptState == nil {
		return
	}

	wasInterrupted = true
	// 如果中断状态为空，直接返回
	if rCtx.interruptState.State == nil {
		return
	}

	state, hasState = rCtx.interruptState.State.(T)
	return
}

// GetResumeContext checks if the current component is the target of a resume operation
// and retrieves any data provided by the user for that resumption.
//
// GetResumeContext 检查当前组件是否为恢复操作的目标，并检索用户为该恢复提供的任何数据。
//
// This function is typically called *after* a component has already determined it is in a
// resumed state by calling GetInterruptState.
//
// 此函数通常在组件通过调用 GetInterruptState 确定其处于恢复状态 *之后* 调用。
//
// It returns three values:
//   - isResumeTarget: A boolean that is true if the current component's address OR any of its
//     descendant addresses was explicitly targeted by a call to Resume() or ResumeWithData().
//     This allows composite components (like tools containing nested graphs) to know they should
//     execute their children to reach the actual resume target.
//   - hasData: A boolean that is true if data was provided for this specific component (i.e., not nil).
//   - data: The typed data provided by the user.
//
// 它返回三个值：
//   - isResumeTarget: 一个布尔值，如果当前组件的地址 或 其任何后代地址被 Resume() 或 ResumeWithData() 显式定位，则为 true。
//     这允许组合组件（如包含嵌套图的工具）知道它们应该执行其子组件以到达实际的恢复目标。
//   - hasData: 一个布尔值，如果为此特定组件提供了数据（即不为 nil），则为 true。
//   - data: 用户提供的类型化数据。
//
// ### How to Use This Function: A Decision Framework
// ### 如何使用此函数：决策框架
//
// The correct usage pattern depends on the application's desired resume strategy.
// 正确的使用模式取决于应用程序所需的恢复策略。
//
// #### Strategy 1: Implicit "Resume All"
// #### 策略 1：隐式 "全部恢复"
// In some use cases, any resume operation implies that *all* interrupted points should proceed.
// For example, if an application's UI only provides a single "Continue" button for a set of
// interruptions. In this model, a component can often just use `GetInterruptState` to see if
// `wasInterrupted` is true and then proceed with its logic, as it can assume it is an intended target.
// It may still call `GetResumeContext` to check for optional data, but the `isResumeFlow` flag is less critical.
// 在某些用例中，任何恢复操作都意味着 *所有* 中断点都应继续。
// 例如，如果应用程序的 UI 仅为一组中断提供单个 "继续" 按钮。
// 在这种模式下，组件通常只需使用 `GetInterruptState` 查看 `wasInterrupted` 是否为 true，然后继续其逻辑，
// 因为它可以假设它是预期的目标。它仍然可以调用 `GetResumeContext` 来检查可选数据，但 `isResumeFlow` 标志不太关键。
//
// #### Strategy 2: Explicit "Targeted Resume" (Most Common)
// #### 策略 2：显式 "定向恢复"（最常见）
// For applications with multiple, distinct interrupt points that must be resumed independently, it is
// crucial to differentiate which point is being resumed. This is the primary use case for the `isResumeTarget` flag.
// 对于具有多个必须独立恢复的不同中断点的应用程序，区分正在恢复哪个点至关重要。这是 `isResumeTarget` 标志的主要用例。
//   - If `isResumeTarget` is `true`: Your component (or one of its descendants) is the target.
//     If `hasData` is true, you are the direct target and should consume the data.
//     If `hasData` is false, a descendant is the target—execute your children to reach it.
//   - If `isResumeTarget` is `false`: Neither you nor your descendants are the target. You MUST
//     re-interrupt (e.g., by returning `StatefulInterrupt(...)`) to preserve your state.
//   - 如果 `isResumeTarget` 为 `true`：您的组件（或其后代之一）是目标。
//     如果 `hasData` 为 true，则是直接目标，应使用该数据。
//     如果 `hasData` 为 false，则后代是目标——执行子组件以到达它。
//   - 如果 `isResumeTarget` 为 `false`：您和您的后代都不是目标。您必须
//     重新中断（例如，通过返回 `StatefulInterrupt(...)`）以保留您的状态。
//
// ### Guidance for Composite Components
// ### 组合组件指南
//
// Composite components (like `Graph` or other `Runnable`s that contain sub-processes) have a dual role:
// 组合组件（如 `Graph` 或其他包含子进程的 `Runnable`）具有双重角色：
//  1. Check for Self-Targeting: A composite component can itself be the target of a resume
//     operation, for instance, to modify its internal state. It may call `GetResumeContext`
//     to check for data targeted at its own address.
//  1. 检查自身目标：组合组件本身可以是恢复操作的目标，例如修改其内部状态。
//     它可以调用 `GetResumeContext` 来检查针对其自身地址的数据。
//  2. Act as a Conduit: After checking for itself, its primary role is to re-execute its children,
//     allowing the resume context to flow down to them. It must not consume a resume signal
//     intended for one of its descendants.
//  2. 充当管道：在检查自身后，其主要作用是重新执行其子组件，允许恢复上下文流向它们。
//     它不得使用旨在用于其后代之一的恢复信号。
func GetResumeContext[T any](ctx context.Context) (isResumeTarget bool, hasData bool, data T) {
	rCtx, ok := getRunCtx(ctx)
	if !ok {
		return
	}

	isResumeTarget = rCtx.isResumeTarget
	if !isResumeTarget {
		return
	}

	// It is a resume flow, now check for data
	// 这是一个恢复流程，现在检查数据
	if rCtx.resumeData == nil {
		return // hasData is false
	}

	data, hasData = rCtx.resumeData.(T)
	return
}

// getRunCtx 从上下文获取内部执行信息
func getRunCtx(ctx context.Context) (*addrCtx, bool) {
	rCtx, ok := ctx.Value(addrCtxKey{}).(*addrCtx)
	return rCtx, ok
}
