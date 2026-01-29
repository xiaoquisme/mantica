import type { AgentMessage } from "@mariozechner/pi-agent-core";
import type { Model } from "@mariozechner/pi-ai";
import {
  estimateMessagesTokens,
  compactMessagesTokenAware,
  estimateTokenUsage,
  shouldCompact as shouldCompactTokens,
  compactMessagesWithSummary,
  compactMessagesWithChunkedSummary,
  COMPACTION_TARGET_RATIO,
  MIN_KEEP_MESSAGES,
} from "../context-window/index.js";

export type CompactionResult = {
  kept: AgentMessage[];
  removedCount: number;
  /** Token 感知模式下的额外信息 */
  tokensRemoved?: number | undefined;
  tokensKept?: number | undefined;
  /** 摘要模式下生成的摘要 */
  summary?: string | undefined;
  reason: "count" | "tokens" | "summary";
};

/**
 * 基于消息数量的简单压缩（旧逻辑，保持向后兼容）
 */
export function compactMessagesByCount(
  messages: AgentMessage[],
  maxMessages: number,
  keepLast: number,
): CompactionResult | null {
  if (messages.length <= maxMessages) return null;
  const kept = messages.slice(-keepLast);
  return {
    kept,
    removedCount: messages.length - kept.length,
    reason: "count",
  };
}

/**
 * 基于 token 的智能压缩
 */
export function compactMessagesByTokens(
  messages: AgentMessage[],
  availableTokens: number,
  options?: {
    targetRatio?: number;
    minKeepMessages?: number;
  },
): CompactionResult | null {
  const result = compactMessagesTokenAware(messages, availableTokens, options);
  if (!result) return null;

  return {
    kept: result.kept,
    removedCount: result.removedCount,
    tokensRemoved: result.tokensRemoved,
    tokensKept: result.tokensKept,
    reason: "tokens",
  };
}

/** 同步压缩选项（count/tokens 模式） */
export type SyncCompactionOptions = {
  mode: "count" | "tokens";
  // count 模式参数
  maxMessages?: number | undefined;
  keepLast?: number | undefined;
  // tokens 模式参数
  contextWindowTokens?: number | undefined;
  systemPrompt?: string | undefined;
  reserveTokens?: number | undefined;
  targetRatio?: number | undefined;
  minKeepMessages?: number | undefined;
};

/** 摘要压缩选项（summary 模式） */
export type SummaryCompactionOptions = {
  mode: "summary";
  // 必需参数
  model: Model<any>;
  apiKey: string;
  // tokens 模式参数（复用）
  contextWindowTokens?: number | undefined;
  systemPrompt?: string | undefined;
  reserveTokens?: number | undefined;
  targetRatio?: number | undefined;
  minKeepMessages?: number | undefined;
  // summary 特有参数
  customInstructions?: string | undefined;
  previousSummary?: string | undefined;
  signal?: AbortSignal | undefined;
  maxChunkTokens?: number | undefined;
};

export type CompactionOptions = SyncCompactionOptions | SummaryCompactionOptions;

/**
 * 统一的压缩入口（同步版本，用于 count/tokens 模式）
 *
 * 根据模式选择压缩策略
 */
export function compactMessages(
  messages: AgentMessage[],
  options: SyncCompactionOptions,
): CompactionResult | null {
  if (options.mode === "count") {
    return compactMessagesByCount(
      messages,
      options.maxMessages ?? 80,
      options.keepLast ?? 60,
    );
  }

  // Token 模式
  const contextWindowTokens = options.contextWindowTokens ?? 200_000;
  const estimation = estimateTokenUsage({
    messages,
    systemPrompt: options.systemPrompt,
    contextWindowTokens,
    reserveTokens: options.reserveTokens,
  });

  // 检查是否需要压缩
  if (!shouldCompactTokens(estimation)) {
    return null;
  }

  return compactMessagesByTokens(messages, estimation.availableTokens, {
    targetRatio: options.targetRatio ?? COMPACTION_TARGET_RATIO,
    minKeepMessages: options.minKeepMessages ?? MIN_KEEP_MESSAGES,
  });
}

/**
 * 摘要式压缩（异步版本）
 *
 * 使用 LLM 生成历史消息的摘要
 */
export async function compactMessagesAsync(
  messages: AgentMessage[],
  options: SummaryCompactionOptions,
): Promise<CompactionResult | null> {
  const contextWindowTokens = options.contextWindowTokens ?? 200_000;
  const estimation = estimateTokenUsage({
    messages,
    systemPrompt: options.systemPrompt,
    contextWindowTokens,
    reserveTokens: options.reserveTokens,
  });

  // 检查是否需要压缩
  if (!shouldCompactTokens(estimation)) {
    return null;
  }

  // 使用分块摘要处理超大历史
  const result = await compactMessagesWithChunkedSummary({
    messages,
    model: options.model,
    apiKey: options.apiKey,
    availableTokens: estimation.availableTokens,
    targetRatio: options.targetRatio ?? COMPACTION_TARGET_RATIO,
    minKeepMessages: options.minKeepMessages ?? MIN_KEEP_MESSAGES,
    reserveTokens: options.reserveTokens ?? 2048,
    customInstructions: options.customInstructions,
    previousSummary: options.previousSummary,
    signal: options.signal,
    maxChunkTokens: options.maxChunkTokens,
  });

  if (!result) {
    return null;
  }

  return {
    kept: result.kept,
    removedCount: result.removedCount,
    tokensRemoved: result.tokensRemoved,
    tokensKept: result.tokensKept,
    summary: result.summary,
    reason: "summary",
  };
}
