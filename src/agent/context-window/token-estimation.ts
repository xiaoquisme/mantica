/**
 * Token 估算工具
 *
 * 提供消息和系统提示词的 token 计数功能
 */

import type { AgentMessage } from "@mariozechner/pi-agent-core";
import { estimateTokens } from "@mariozechner/pi-coding-agent";
import type { TokenEstimation, TokenAwareCompactionResult } from "./types.js";

/** 安全边界系数，用于补偿估算不准确 */
export const ESTIMATION_SAFETY_MARGIN = 1.2; // 20% buffer

/** 触发 compaction 的利用率阈值 */
export const COMPACTION_TRIGGER_RATIO = 0.8; // 80%

/** Compaction 目标利用率 */
export const COMPACTION_TARGET_RATIO = 0.5; // 50%

/** 最小保留消息数 */
export const MIN_KEEP_MESSAGES = 10;

/**
 * 估算消息数组的总 token 数
 */
export function estimateMessagesTokens(messages: AgentMessage[]): number {
  return messages.reduce((sum, message) => sum + estimateTokens(message), 0);
}

/**
 * 估算系统提示词的 token 数
 */
export function estimateSystemPromptTokens(systemPrompt: string | undefined): number {
  if (!systemPrompt) return 0;
  // 简单估算：约 4 字符 = 1 token（适用于英文/代码混合文本）
  // 中文约 2 字符 = 1 token
  // 取平均值 3
  return Math.ceil(systemPrompt.length / 3);
}

/**
 * 计算完整的 token 使用情况
 */
export function estimateTokenUsage(params: {
  messages: AgentMessage[];
  systemPrompt?: string | undefined;
  contextWindowTokens: number;
  reserveTokens?: number | undefined;
}): TokenEstimation {
  const messageTokens = estimateMessagesTokens(params.messages);
  const systemPromptTokens = estimateSystemPromptTokens(params.systemPrompt);
  const reserve = params.reserveTokens ?? 1024; // 预留给响应生成

  // 可用 token = 总窗口 - 系统提示 - 预留
  const availableTokens = Math.max(
    0,
    params.contextWindowTokens - systemPromptTokens - reserve,
  );

  // 计算利用率（带安全边界）
  const safeMessageTokens = messageTokens * ESTIMATION_SAFETY_MARGIN;
  const utilizationRatio = availableTokens > 0 ? safeMessageTokens / availableTokens : 1;

  return {
    messageTokens,
    systemPromptTokens,
    availableTokens,
    utilizationRatio,
  };
}

/**
 * 判断是否需要 compaction
 */
export function shouldCompact(estimation: TokenEstimation): boolean {
  return estimation.utilizationRatio >= COMPACTION_TRIGGER_RATIO;
}

/**
 * Token 感知的消息压缩
 *
 * 策略：从最旧的消息开始移除，直到达到目标利用率
 */
export function compactMessagesTokenAware(
  messages: AgentMessage[],
  availableTokens: number,
  options?: {
    targetRatio?: number;
    minKeepMessages?: number;
  },
): TokenAwareCompactionResult | null {
  const targetRatio = options?.targetRatio ?? COMPACTION_TARGET_RATIO;
  const minKeep = options?.minKeepMessages ?? MIN_KEEP_MESSAGES;

  if (messages.length <= minKeep) {
    return null; // 消息太少，不压缩
  }

  const currentTokens = estimateMessagesTokens(messages);
  const targetTokens = Math.floor(availableTokens * targetRatio);

  // 如果当前已经在目标内，不需要压缩
  if (currentTokens <= targetTokens) {
    return null;
  }

  // 从后往前保留消息，直到达到目标 token 数
  const kept: AgentMessage[] = [];
  let keptTokens = 0;

  // 反向遍历，保留最新的消息
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i]!;
    const msgTokens = estimateTokens(msg);

    // 检查是否可以添加这条消息
    if (keptTokens + msgTokens <= targetTokens || kept.length < minKeep) {
      kept.unshift(msg);
      keptTokens += msgTokens;
    }

    // 如果已经达到最小保留数且超过目标，停止
    if (kept.length >= minKeep && keptTokens >= targetTokens) {
      break;
    }
  }

  // 如果保留的消息数量不变，说明没有压缩
  if (kept.length >= messages.length) {
    return null;
  }

  const removedCount = messages.length - kept.length;
  const tokensRemoved = currentTokens - keptTokens;

  return {
    kept,
    removedCount,
    tokensRemoved,
    tokensKept: keptTokens,
  };
}

/**
 * 检查单条消息是否过大
 *
 * 如果单条消息超过 context window 的一定比例，可能需要特殊处理
 */
export function isMessageOversized(
  message: AgentMessage,
  contextWindowTokens: number,
  maxRatio: number = 0.5,
): boolean {
  const tokens = estimateTokens(message) * ESTIMATION_SAFETY_MARGIN;
  return tokens > contextWindowTokens * maxRatio;
}
