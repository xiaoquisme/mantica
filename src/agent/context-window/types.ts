/**
 * Context Window Guard - 类型定义
 *
 * 用于管理和验证 LLM 上下文窗口限制
 */

/** Context window 信息来源 */
export type ContextWindowSource = "model" | "config" | "default";

/** Context window 信息 */
export type ContextWindowInfo = {
  /** Token 数量 */
  tokens: number;
  /** 来源 */
  source: ContextWindowSource;
};

/** Context window guard 验证结果 */
export type ContextWindowGuardResult = ContextWindowInfo & {
  /** 是否需要警告（窗口较小） */
  shouldWarn: boolean;
  /** 是否应该阻止运行（窗口太小） */
  shouldBlock: boolean;
};

/** Token 估算结果 */
export type TokenEstimation = {
  /** 消息总 token 数 */
  messageTokens: number;
  /** 系统提示词 token 数 */
  systemPromptTokens: number;
  /** 可用 token 数 */
  availableTokens: number;
  /** 使用率 (0-1) */
  utilizationRatio: number;
};

/** Compaction 结果（带 token 信息） */
export type TokenAwareCompactionResult = {
  /** 保留的消息 */
  kept: import("@mariozechner/pi-agent-core").AgentMessage[];
  /** 移除的消息数量 */
  removedCount: number;
  /** 移除的 token 数 */
  tokensRemoved: number;
  /** 保留的 token 数 */
  tokensKept: number;
};
