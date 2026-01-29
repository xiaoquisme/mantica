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
};
