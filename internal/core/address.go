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

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/internal/generic"
)

// AddressSegmentType defines the type of a segment in an execution address.
// AddressSegmentType 定义执行地址中片段的类型
type AddressSegmentType string

// Address represents a full, hierarchical address to a point in the execution structure.
// Address 表示执行结构中某一点的完整层级地址
type Address []AddressSegment

// String converts an Address into its unique string representation.
// String 将层级地址转换为唯一字符串表示（Type:ID(:SubID);...）
func (p Address) String() string {
	if p == nil {
		return ""
	}
	var sb strings.Builder
	for i, s := range p {
		sb.WriteString(string(s.Type))
		sb.WriteString(":")
		sb.WriteString(s.ID)
		if s.SubID != "" {
			sb.WriteString(":")
			sb.WriteString(s.SubID)
		}
		if i != len(p)-1 {
			sb.WriteString(";")
		}
	}
	return sb.String()
}

// Equals 比较两个层级地址是否完全一致
func (p Address) Equals(other Address) bool {
	if len(p) != len(other) {
		return false
	}
	for i := range p {
		if p[i].Type != other[i].Type || p[i].ID != other[i].ID || p[i].SubID != other[i].SubID {
			return false
		}
	}
	return true
}

// AddressSegment represents a single segment in the hierarchical address of an execution point.
// A sequence of AddressSegments uniquely identifies a location within a potentially nested structure.
// AddressSegment 执行点层级地址中的单个片段
type AddressSegment struct {
	// ID is the unique identifier for this segment, e.g., the node's key or the tool's name.
	// ID 片段唯一标识符（如节点 Key 或工具名称）
	ID string
	// Type indicates whether this address segment is a graph node, a tool call, an agent, etc.
	// Type 地址段类型（节点、工具、Agent 等）
	Type AddressSegmentType
	// In some cases, ID alone are not unique enough, we need this SubID to guarantee uniqueness.
	// e.g. parallel tool calls with the same name but different tool call IDs.
	// SubID 子 ID，用于保证唯一性（如并行工具调用）
	SubID string
}

// addrCtxKey 上下文中存储 addrCtx 的键
type addrCtxKey struct{}

// addrCtx 内部执行上下文，包含地址、中断状态和恢复数据
type addrCtx struct {
	addr           Address
	interruptState *InterruptState
	isResumeTarget bool
	resumeData     any
}

// globalResumeInfoKey 上下文中存储全局恢复信息的键
type globalResumeInfoKey struct{}

// globalResumeInfo 全局恢复信息，包含所有恢复点的数据和状态
type globalResumeInfo struct {
	mu                sync.Mutex
	id2ResumeData     map[string]any
	id2ResumeDataUsed map[string]bool
	id2State          map[string]InterruptState
	id2StateUsed      map[string]bool
	id2Addr           map[string]Address
}

// GetCurrentAddress returns the hierarchical address of the currently executing component.
// The address is a sequence of segments, each identifying a structural part of the execution
// like an agent, a graph node, or a tool call. This can be useful for logging or debugging.
// GetCurrentAddress 返回当前执行组件的层级地址（用于日志/诊断）
func GetCurrentAddress(ctx context.Context) Address {
	if p, ok := ctx.Value(addrCtxKey{}).(*addrCtx); ok {
		return p.addr
	}

	return nil
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
// AppendAddressSegment 为子组件创建新的执行上下文（扩展地址，并注入中断/恢复信息）
// - 使用：在进入子组件（节点/工具等）时调用，形成新的地址段上下文
// - 逻辑：根据当前地址生成新地址；若全局恢复信息存在，注入匹配的中断状态与恢复数据
func AppendAddressSegment(ctx context.Context, segType AddressSegmentType, segID string,
	subID string) context.Context {
	// get current address
	currentAddress := GetCurrentAddress(ctx)
	if len(currentAddress) == 0 {
		currentAddress = []AddressSegment{
			{
				Type:  segType,
				ID:    segID,
				SubID: subID,
			},
		}
	} else {
		newAddress := make([]AddressSegment, len(currentAddress)+1)
		copy(newAddress, currentAddress)
		newAddress[len(newAddress)-1] = AddressSegment{
			Type:  segType,
			ID:    segID,
			SubID: subID,
		}
		currentAddress = newAddress
	}

	runCtx := &addrCtx{
		addr: currentAddress,
	}

	rInfo, hasRInfo := getResumeInfo(ctx)
	if !hasRInfo {
		return context.WithValue(ctx, addrCtxKey{}, runCtx)
	}

	var id string
	for id_, addr := range rInfo.id2Addr {
		if addr.Equals(currentAddress) {
			rInfo.mu.Lock()
			if used, ok := rInfo.id2StateUsed[id_]; !ok || !used {
				runCtx.interruptState = generic.PtrOf(rInfo.id2State[id_])
				rInfo.id2StateUsed[id_] = true
				id = id_
				rInfo.mu.Unlock()
				break
			}
			rInfo.mu.Unlock()
		}
	}

	// take from globalResumeInfo the data for the new address if there is any
	rInfo.mu.Lock()
	defer rInfo.mu.Unlock()
	used := rInfo.id2ResumeDataUsed[id]
	if !used {
		rData, existed := rInfo.id2ResumeData[id]
		if existed {
			rInfo.id2ResumeDataUsed[id] = true
			runCtx.resumeData = rData
			runCtx.isResumeTarget = true
		}
	}

	// Also mark as resume target if any descendant address is a resume target.
	// This allows composite components (e.g., a tool containing a nested graph) to know
	// they should execute their children to reach the actual resume target.
	// We only consider descendants whose resume data has not yet been consumed.
	if !runCtx.isResumeTarget {
		for id_, addr := range rInfo.id2Addr {
			if len(addr) > len(currentAddress) && addr[:len(currentAddress)].Equals(currentAddress) {
				if !rInfo.id2ResumeDataUsed[id_] {
					runCtx.isResumeTarget = true
					break
				}
			}
		}
	}

	return context.WithValue(ctx, addrCtxKey{}, runCtx)
}

// GetNextResumptionPoints finds the immediate child resumption points for a given parent address.
// GetNextResumptionPoints 返回当前地址的“下一层”可恢复子节点集合（ID->true）
func GetNextResumptionPoints(ctx context.Context) (map[string]bool, error) {
	parentAddr := GetCurrentAddress(ctx)

	rInfo, exists := getResumeInfo(ctx)
	if !exists {
		return nil, fmt.Errorf("GetNextResumptionPoints: failed to get resume info from context")
	}

	nextPoints := make(map[string]bool)
	parentAddrLen := len(parentAddr)

	for _, addr := range rInfo.id2Addr {
		// Check if addr is a potential child (must be longer than parent)
		if len(addr) <= parentAddrLen {
			continue
		}

		// Check if it has the parent address as a prefix
		var isPrefix bool
		if parentAddrLen == 0 {
			isPrefix = true
		} else {
			isPrefix = addr[:parentAddrLen].Equals(parentAddr)
		}

		if !isPrefix {
			continue
		}

		// We are looking for immediate children.
		// The address of an immediate child should be one segment longer.
		childAddr := addr[parentAddrLen : parentAddrLen+1]
		childID := childAddr[0].ID

		// Avoid adding duplicates.
		if _, ok := nextPoints[childID]; !ok {
			nextPoints[childID] = true
		}
	}

	return nextPoints, nil
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
// BatchResumeWithData 向上下文注入“目标恢复数据”（显式点名恢复策略）
// - 参数：resumeData 为 ID->数据 的映射，ID 为地址字符串
// - 行为：若上下文未含全局恢复信息，创建并复制用户映射；否则合并注入
func BatchResumeWithData(ctx context.Context, resumeData map[string]any) context.Context {
	rInfo, ok := ctx.Value(globalResumeInfoKey{}).(*globalResumeInfo)
	if !ok {
		// Create a new globalResumeInfo and copy the map to prevent external mutation.
		newMap := make(map[string]any, len(resumeData))
		for k, v := range resumeData {
			newMap[k] = v
		}
		return context.WithValue(ctx, globalResumeInfoKey{}, &globalResumeInfo{
			id2ResumeData:     newMap,
			id2ResumeDataUsed: make(map[string]bool),
			id2StateUsed:      make(map[string]bool),
		})
	}

	rInfo.mu.Lock()
	defer rInfo.mu.Unlock()
	if rInfo.id2ResumeData == nil {
		rInfo.id2ResumeData = make(map[string]any)
	}
	for id, data := range resumeData {
		rInfo.id2ResumeData[id] = data
	}
	return ctx
}

func PopulateInterruptState(ctx context.Context, id2Addr map[string]Address,
	id2State map[string]InterruptState) context.Context {
	// Populate 全局的中断地址与状态表，同时尝试与当前运行上下文匹配注入
	rInfo, ok := ctx.Value(globalResumeInfoKey{}).(*globalResumeInfo)
	if ok {
		if rInfo.id2Addr == nil {
			rInfo.id2Addr = make(map[string]Address)
		}
		for id, addr := range id2Addr {
			rInfo.id2Addr[id] = addr
		}
		rInfo.id2State = id2State
	} else {
		rInfo = &globalResumeInfo{
			id2Addr:           id2Addr,
			id2State:          id2State,
			id2StateUsed:      make(map[string]bool),
			id2ResumeDataUsed: make(map[string]bool),
		}
		ctx = context.WithValue(ctx, globalResumeInfoKey{}, rInfo)
	}

	runCtx, ok := getRunCtx(ctx)
	if ok {
		for id_, addr := range id2Addr {
			if addr.Equals(runCtx.addr) {
				if used, ok := rInfo.id2StateUsed[id_]; !ok || !used {
					runCtx.interruptState = generic.PtrOf(rInfo.id2State[id_])
					rInfo.mu.Lock()
					rInfo.id2StateUsed[id_] = true
					rInfo.mu.Unlock()
				}

				if used, ok := rInfo.id2ResumeDataUsed[id_]; !ok || !used {
					runCtx.isResumeTarget = true
					runCtx.resumeData = rInfo.id2ResumeData[id_]
					rInfo.mu.Lock()
					rInfo.id2ResumeDataUsed[id_] = true
					rInfo.mu.Unlock()
				}

				break
			}
		}
	}

	return ctx
}

func getResumeInfo(ctx context.Context) (*globalResumeInfo, bool) {
	info, ok := ctx.Value(globalResumeInfoKey{}).(*globalResumeInfo)
	return info, ok
}

// InterruptInfo 中断信息结构
type InterruptInfo struct {
	// Info 用户定义的中断信息
	Info any
	// IsRootCause 是否为中断的根因
	IsRootCause bool
}

// String 返回用户可读的中断信息字符串
func (i *InterruptInfo) String() string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("interrupt info: Info=%v, IsRootCause=%v", i.Info, i.IsRootCause)
}
