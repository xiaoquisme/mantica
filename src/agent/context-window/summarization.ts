/**
 * 摘要式 Compaction
 *
 * 使用 LLM 生成历史消息的摘要，而不是简单截断
 */

import type { AgentMessage } from "@mariozechner/pi-agent-core";
import { generateSummary, estimateTokens } from "@mariozechner/pi-coding-agent";
import type { Model } from "@mariozechner/pi-ai";
import { estimateMessagesTokens } from "./token-estimation.js";

/** 摘要 Compaction 结果 */
export type SummaryCompactionResult = {
  /** 保留的消息（包含摘要消息） */
  kept: AgentMessage[];
  /** 移除的消息数量 */
  removedCount: number;
  /** 移除的 token 数 */
  tokensRemoved: number;
  /** 保留的 token 数 */
  tokensKept: number;
  /** 生成的摘要 */
  summary: string;
  /** compaction 原因 */
  reason: "summary";
};

/** 摘要 Compaction 参数 */
export type SummaryCompactionParams = {
  /** 消息列表 */
  messages: AgentMessage[];
  /** LLM Model（用于生成摘要） */
  model: Model<any>;
  /** API Key */
  apiKey: string;
  /** 可用 token 数 */
  availableTokens: number;
  /** 目标利用率 (0-1)，默认 0.5 */
  targetRatio?: number | undefined;
  /** 最少保留消息数，默认 10 */
  minKeepMessages?: number | undefined;
  /** 预留给摘要生成的 token 数，默认 2048 */
  reserveTokens?: number | undefined;
  /** 自定义摘要指令 */
  customInstructions?: string | undefined;
  /** 之前的摘要（用于增量更新） */
  previousSummary?: string | undefined;
  /** AbortSignal */
  signal?: AbortSignal | undefined;
};

/** 默认摘要提示词 */
const DEFAULT_SUMMARY_INSTRUCTIONS = `Summarize the conversation history concisely, focusing on:
- Key decisions made
- Important context and constraints
- Open questions or TODOs
- Technical details that may be needed later

Keep the summary concise but complete. Use bullet points for clarity.`;

/**
 * 将消息分割为需要摘要的部分和保留的部分
 */
export function splitMessagesForSummary(
  messages: AgentMessage[],
  availableTokens: number,
  options?: {
    targetRatio?: number | undefined;
    minKeepMessages?: number | undefined;
  },
): { toSummarize: AgentMessage[]; toKeep: AgentMessage[] } | null {
  const targetRatio = options?.targetRatio ?? 0.5;
  const minKeep = options?.minKeepMessages ?? 10;

  if (messages.length <= minKeep) {
    return null; // 消息太少，不需要压缩
  }

  const totalTokens = estimateMessagesTokens(messages);
  const targetTokens = Math.floor(availableTokens * targetRatio);

  // 如果当前已经在目标内，不需要压缩
  if (totalTokens <= targetTokens) {
    return null;
  }

  // 从后往前保留消息
  const toKeep: AgentMessage[] = [];
  let keptTokens = 0;

  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i]!;
    const msgTokens = estimateTokens(msg);

    // 检查是否可以添加这条消息
    if (keptTokens + msgTokens <= targetTokens || toKeep.length < minKeep) {
      toKeep.unshift(msg);
      keptTokens += msgTokens;
    }

    // 如果已经达到最小保留数且超过目标，停止
    if (toKeep.length >= minKeep && keptTokens >= targetTokens) {
      break;
    }
  }

  // 需要摘要的消息
  const toSummarize = messages.slice(0, messages.length - toKeep.length);

  if (toSummarize.length === 0) {
    return null;
  }

  return { toSummarize, toKeep };
}

/**
 * 创建摘要消息
 */
function createSummaryMessage(summary: string, previousSummary?: string): AgentMessage {
  const content = previousSummary
    ? `## Previous Context Summary\n${previousSummary}\n\n## Recent Context Summary\n${summary}`
    : `## Conversation Summary\n${summary}`;

  return {
    role: "user",
    content: `[System Note: The following is a summary of the earlier conversation history that has been compacted to save context space.]\n\n${content}\n\n[End of Summary]`,
    timestamp: Date.now(),
  };
}

/**
 * 执行摘要式 Compaction
 *
 * 使用 LLM 生成历史消息的摘要，然后将摘要和最近的消息组合
 */
export async function compactMessagesWithSummary(
  params: SummaryCompactionParams,
): Promise<SummaryCompactionResult | null> {
  const {
    messages,
    model,
    apiKey,
    availableTokens,
    targetRatio,
    minKeepMessages,
    reserveTokens = 2048,
    customInstructions,
    previousSummary,
    signal,
  } = params;

  // 分割消息
  const split = splitMessagesForSummary(messages, availableTokens, {
    targetRatio,
    minKeepMessages,
  });

  if (!split) {
    return null;
  }

  const { toSummarize, toKeep } = split;

  // 生成摘要
  const instructions = customInstructions || DEFAULT_SUMMARY_INSTRUCTIONS;
  const summary = await generateSummary(
    toSummarize,
    model,
    reserveTokens,
    apiKey,
    signal,
    instructions,
    previousSummary,
  );

  // 创建摘要消息
  const summaryMessage = createSummaryMessage(summary, previousSummary);

  // 组合结果
  const kept = [summaryMessage, ...toKeep];

  const tokensRemoved = estimateMessagesTokens(toSummarize);
  const tokensKept = estimateMessagesTokens(kept);

  return {
    kept,
    removedCount: toSummarize.length,
    tokensRemoved,
    tokensKept,
    summary,
    reason: "summary",
  };
}

/**
 * 分块生成摘要（用于超大历史）
 *
 * 当历史太大时，分块生成摘要然后合并
 */
export async function compactMessagesWithChunkedSummary(
  params: SummaryCompactionParams & {
    maxChunkTokens?: number | undefined;
  },
): Promise<SummaryCompactionResult | null> {
  const {
    messages,
    model,
    apiKey,
    availableTokens,
    targetRatio,
    minKeepMessages,
    reserveTokens = 2048,
    customInstructions,
    previousSummary,
    signal,
    maxChunkTokens = 50000,
  } = params;

  // 分割消息
  const split = splitMessagesForSummary(messages, availableTokens, {
    targetRatio,
    minKeepMessages,
  });

  if (!split) {
    return null;
  }

  const { toSummarize, toKeep } = split;

  // 如果需要摘要的消息不多，直接摘要
  const toSummarizeTokens = estimateMessagesTokens(toSummarize);
  if (toSummarizeTokens <= maxChunkTokens) {
    return compactMessagesWithSummary(params);
  }

  // 分块处理
  const chunks: AgentMessage[][] = [];
  let currentChunk: AgentMessage[] = [];
  let currentTokens = 0;

  for (const msg of toSummarize) {
    const msgTokens = estimateTokens(msg);

    if (currentTokens + msgTokens > maxChunkTokens && currentChunk.length > 0) {
      chunks.push(currentChunk);
      currentChunk = [];
      currentTokens = 0;
    }

    currentChunk.push(msg);
    currentTokens += msgTokens;
  }

  if (currentChunk.length > 0) {
    chunks.push(currentChunk);
  }

  // 为每个块生成摘要
  const instructions = customInstructions || DEFAULT_SUMMARY_INSTRUCTIONS;
  const chunkSummaries: string[] = [];

  let runningContext = previousSummary;
  for (const chunk of chunks) {
    const chunkSummary = await generateSummary(
      chunk,
      model,
      reserveTokens,
      apiKey,
      signal,
      instructions,
      runningContext,
    );
    chunkSummaries.push(chunkSummary);
    runningContext = chunkSummary;
  }

  // 最终摘要就是最后一个块的摘要（已经包含了之前的上下文）
  const finalSummary = chunkSummaries[chunkSummaries.length - 1] ?? "";

  // 创建摘要消息
  const summaryMessage = createSummaryMessage(finalSummary);

  // 组合结果
  const kept = [summaryMessage, ...toKeep];

  const tokensRemoved = estimateMessagesTokens(toSummarize);
  const tokensKept = estimateMessagesTokens(kept);

  return {
    kept,
    removedCount: toSummarize.length,
    tokensRemoved,
    tokensKept,
    summary: finalSummary,
    reason: "summary",
  };
}
