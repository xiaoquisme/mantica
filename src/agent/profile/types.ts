/**
 * Agent Profile 类型定义
 */

/** Profile 文件名常量 */
export const PROFILE_FILES = {
  soul: "soul.md",
  identity: "identity.md",
  tools: "tools.md",
  memory: "memory.md",
  bootstrap: "bootstrap.md",
} as const;

/** Agent Profile 配置 */
export interface AgentProfile {
  /** Profile ID */
  id: string;
  /** 人格约束 - 定义 agent 的行为边界和风格 */
  soul?: string | undefined;
  /** 身份信息 - agent 的名称和自我认知 */
  identity?: string | undefined;
  /** 自定义工具描述 - 额外的工具使用说明 */
  tools?: string | undefined;
  /** 持久记忆 - 长期知识库 */
  memory?: string | undefined;
  /** 初始上下文 - 每次对话的引导信息 */
  bootstrap?: string | undefined;
}

/** Profile Manager 选项 */
export interface ProfileManagerOptions {
  /** Profile ID */
  profileId: string;
  /** 基础目录，默认 ~/.super-multica/agent-profiles */
  baseDir?: string | undefined;
}

/** 创建 Profile 的选项 */
export interface CreateProfileOptions {
  /** 基础目录 */
  baseDir?: string | undefined;
  /** 是否使用默认模板初始化 */
  useTemplates?: boolean | undefined;
}
