/**
 * Context Window Guard - 上下文窗口验证
 *
 * 在 agent 运行前验证 context window 是否足够，防止 token 溢出
 */

import type { ContextWindowInfo, ContextWindowGuardResult, ContextWindowSource } from "./types.js";

/** 硬性最小 token 数，低于此值将阻止运行 */
export const CONTEXT_WINDOW_HARD_MIN_TOKENS = 16_000;

/** 警告阈值，低于此值会发出警告 */
export const CONTEXT_WINDOW_WARN_BELOW_TOKENS = 32_000;

/** 默认 context window（当无法获取时） */
export const DEFAULT_CONTEXT_TOKENS = 200_000;

/**
 * 标准化为正整数
 */
function normalizePositiveInt(value: unknown): number | null {
  if (typeof value !== "number" || !Number.isFinite(value)) return null;
  const int = Math.floor(value);
  return int > 0 ? int : null;
}

/**
 * 解析 context window 信息
 *
 * 优先级：model > config > default
 */
export function resolveContextWindowInfo(params: {
  /** Model 的 contextWindow 属性 */
  modelContextWindow?: number | undefined;
  /** 配置中指定的 context tokens */
  configContextTokens?: number | undefined;
  /** 默认值 */
  defaultTokens?: number | undefined;
}): ContextWindowInfo {
  // 1. 尝试从 model 获取
  const fromModel = normalizePositiveInt(params.modelContextWindow);
  if (fromModel) {
    return { tokens: fromModel, source: "model" };
  }

  // 2. 尝试从配置获取
  const fromConfig = normalizePositiveInt(params.configContextTokens);
  if (fromConfig) {
    return { tokens: fromConfig, source: "config" };
  }

  // 3. 使用默认值
  return {
    tokens: Math.floor(params.defaultTokens ?? DEFAULT_CONTEXT_TOKENS),
    source: "default",
  };
}

/**
 * 评估 context window guard
 *
 * 返回是否需要警告或阻止运行
 */
export function evaluateContextWindowGuard(params: {
  info: ContextWindowInfo;
  warnBelowTokens?: number | undefined;
  hardMinTokens?: number | undefined;
}): ContextWindowGuardResult {
  const warnBelow = Math.max(
    1,
    Math.floor(params.warnBelowTokens ?? CONTEXT_WINDOW_WARN_BELOW_TOKENS),
  );
  const hardMin = Math.max(
    1,
    Math.floor(params.hardMinTokens ?? CONTEXT_WINDOW_HARD_MIN_TOKENS),
  );
  const tokens = Math.max(0, Math.floor(params.info.tokens));

  return {
    ...params.info,
    tokens,
    shouldWarn: tokens > 0 && tokens < warnBelow,
    shouldBlock: tokens > 0 && tokens < hardMin,
  };
}

/**
 * 完整的 context window guard 流程
 *
 * 解析 + 评估一步完成
 */
export function checkContextWindow(params: {
  modelContextWindow?: number | undefined;
  configContextTokens?: number | undefined;
  defaultTokens?: number | undefined;
  warnBelowTokens?: number | undefined;
  hardMinTokens?: number | undefined;
}): ContextWindowGuardResult {
  const info = resolveContextWindowInfo({
    modelContextWindow: params.modelContextWindow,
    configContextTokens: params.configContextTokens,
    defaultTokens: params.defaultTokens,
  });

  return evaluateContextWindowGuard({
    info,
    warnBelowTokens: params.warnBelowTokens,
    hardMinTokens: params.hardMinTokens,
  });
}
