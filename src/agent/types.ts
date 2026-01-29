import type { ThinkingLevel } from "@mariozechner/pi-agent-core";

export type AgentRunResult = {
  text: string;
  error?: string | undefined;
};

export type AgentLogger = {
  stdout?: NodeJS.WritableStream | undefined;
  stderr?: NodeJS.WritableStream | undefined;
};

export type AgentOptions = {
  /** Agent Profile ID - 加载预定义的身份、人格、记忆等配置 */
  profileId?: string | undefined;
  /** Profile 基础目录，默认 ~/.super-multica/agent-profiles */
  profileBaseDir?: string | undefined;
  provider?: string | undefined;
  model?: string | undefined;
  /** System prompt，如果设置了 profileId 会自动从 profile 构建 */
  systemPrompt?: string | undefined;
  thinkingLevel?: ThinkingLevel | undefined;
  /** 命令执行目录 */
  cwd?: string | undefined;
  sessionId?: string | undefined;
  logger?: AgentLogger | undefined;

  // === Context Window Guard 配置 ===
  /** 手动指定 context window token 数（覆盖 model 的值） */
  contextWindowTokens?: number | undefined;
  /** 预留给响应生成的 token 数，默认 1024 */
  reserveTokens?: number | undefined;
  /**
   * Compaction 模式:
   * - "count": 使用旧的消息计数
   * - "tokens": 使用 token 感知（默认）
   * - "summary": 使用 LLM 生成摘要
   */
  compactionMode?: "count" | "tokens" | "summary" | undefined;
  /** Compaction 目标利用率 (0-1)，默认 0.5 */
  compactionTargetRatio?: number | undefined;
  /** 最小保留消息数，默认 10 */
  minKeepMessages?: number | undefined;

  // === Summary Compaction 配置 ===
  /** 自定义摘要生成指令 */
  summaryInstructions?: string | undefined;
};
