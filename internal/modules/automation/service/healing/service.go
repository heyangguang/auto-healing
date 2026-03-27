// Package healing 自愈引擎服务
//
// 本包包含自愈流程的核心业务逻辑：
// - FlowExecutor: 流程执行器，负责执行自愈流程
// - Matcher: 规则匹配器，负责匹配工单与规则
// - Scheduler: 调度器，负责定期扫描和触发
//
// 入口文件，聚合各组件供外部调用
package healing

// 重新导出核心组件，方便外部使用
//
// 使用示例:
//   executor := healing.NewFlowExecutor()
//   matcher := healing.NewMatcher()
//   scheduler := healing.NewScheduler()
