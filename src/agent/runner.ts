import { Agent as PiAgentCore, type AgentEvent, type AgentMessage } from "@mariozechner/pi-agent-core";
import { v7 as uuidv7 } from "uuid";
import type { AgentOptions, AgentRunResult } from "./types.js";
import { createAgentOutput } from "./output.js";
import { resolveModel, resolveTools } from "./tools.js";
import { SessionManager } from "./session/session-manager.js";
import { ProfileManager } from "./profile/index.js";
import {
  checkContextWindow,
  DEFAULT_CONTEXT_TOKENS,
  type ContextWindowGuardResult,
} from "./context-window/index.js";

/**
 * 根据 provider 获取 API Key
 */
function resolveApiKey(provider: string): string | undefined {
  const providerEnvMap: Record<string, string> = {
    openai: "OPENAI_API_KEY",
    anthropic: "ANTHROPIC_API_KEY",
    google: "GOOGLE_API_KEY",
    "google-genai": "GOOGLE_API_KEY",
    kimi: "MOONSHOT_API_KEY",
    "kimi-coding": "MOONSHOT_API_KEY",
    deepseek: "DEEPSEEK_API_KEY",
    groq: "GROQ_API_KEY",
    mistral: "MISTRAL_API_KEY",
    together: "TOGETHER_API_KEY",
  };

  const envVar = providerEnvMap[provider];
  if (envVar) {
    return process.env[envVar];
  }

  // 尝试通用格式: PROVIDER_API_KEY
  const normalizedProvider = provider.toUpperCase().replace(/-/g, "_");
  return process.env[`${normalizedProvider}_API_KEY`];
}

export class Agent {
  private readonly agent: PiAgentCore;
  private readonly output;
  private readonly session: SessionManager;
  private readonly profile?: ProfileManager;
  private readonly contextWindowGuard: ContextWindowGuardResult;

  /** 当前会话 ID */
  readonly sessionId: string;

  constructor(options: AgentOptions = {}) {
    const stdout = options.logger?.stdout ?? process.stdout;
    const stderr = options.logger?.stderr ?? process.stderr;
    this.output = createAgentOutput({ stdout, stderr });

    this.agent = new PiAgentCore();

    // 加载 Agent Profile（如果指定了 profileId）
    let systemPrompt: string | undefined;
    if (options.profileId) {
      this.profile = new ProfileManager({
        profileId: options.profileId,
        baseDir: options.profileBaseDir,
      });
      systemPrompt = this.profile.buildSystemPrompt();
      if (systemPrompt) {
        this.agent.setSystemPrompt(systemPrompt);
      }
    } else if (options.systemPrompt) {
      // 直接使用传入的 systemPrompt
      systemPrompt = options.systemPrompt;
      this.agent.setSystemPrompt(options.systemPrompt);
    }

    this.sessionId = options.sessionId ?? uuidv7();

    // 解析 model（用于获取 context window）
    const storedMeta = (() => {
      // 临时创建 session 获取 meta，避免循环依赖
      const tempSession = new SessionManager({ sessionId: this.sessionId });
      return tempSession.getMeta();
    })();

    const model = options.provider && options.model ? resolveModel(options) : resolveModel({
      ...options,
      provider: storedMeta?.provider,
      model: storedMeta?.model,
    });

    // === Context Window Guard ===
    this.contextWindowGuard = checkContextWindow({
      modelContextWindow: model.contextWindow,
      configContextTokens: options.contextWindowTokens,
      defaultTokens: DEFAULT_CONTEXT_TOKENS,
    });

    // 警告：context window 较小
    if (this.contextWindowGuard.shouldWarn) {
      stderr.write(
        `[Context Window Guard] WARNING: Low context window: ${this.contextWindowGuard.tokens} tokens (source: ${this.contextWindowGuard.source})\n`,
      );
    }

    // 阻止：context window 太小
    if (this.contextWindowGuard.shouldBlock) {
      throw new Error(
        `[Context Window Guard] Context window too small: ${this.contextWindowGuard.tokens} tokens. ` +
          `Minimum required: 16,000 tokens. Please use a model with a larger context window.`,
      );
    }

    // 确定 compaction 模式
    const compactionMode = options.compactionMode ?? "tokens"; // 默认使用 token 模式

    // 获取 API Key（用于 summary 模式）
    const apiKey = compactionMode === "summary" ? resolveApiKey(model.provider) : undefined;

    // 创建 SessionManager（带 context window 配置）
    this.session = new SessionManager({
      sessionId: this.sessionId,
      compactionMode,
      // Token 模式参数
      contextWindowTokens: this.contextWindowGuard.tokens,
      systemPrompt,
      reserveTokens: options.reserveTokens,
      targetRatio: options.compactionTargetRatio,
      minKeepMessages: options.minKeepMessages,
      // Summary 模式参数
      model: compactionMode === "summary" ? model : undefined,
      apiKey,
      customInstructions: options.summaryInstructions,
    });

    if (!options.thinkingLevel && storedMeta?.thinkingLevel) {
      this.agent.setThinkingLevel(storedMeta.thinkingLevel as any);
    } else if (options.thinkingLevel) {
      this.agent.setThinkingLevel(options.thinkingLevel);
    }

    this.agent.setModel(model);
    this.agent.setTools(resolveTools(options));

    const restoredMessages = this.session.loadMessages();
    if (restoredMessages.length > 0) {
      this.agent.replaceMessages(restoredMessages);
    }

    this.session.saveMeta({
      provider: this.agent.state.model?.provider,
      model: this.agent.state.model?.id,
      thinkingLevel: this.agent.state.thinkingLevel,
      contextWindowTokens: this.contextWindowGuard.tokens,
    });

    this.agent.subscribe((event: AgentEvent) => {
      this.output.handleEvent(event);
      this.handleSessionEvent(event);
    });
  }

  async run(prompt: string): Promise<AgentRunResult> {
    this.output.state.lastAssistantText = "";
    await this.agent.prompt(prompt);
    return { text: this.output.state.lastAssistantText, error: this.agent.state.error };
  }

  private handleSessionEvent(event: AgentEvent) {
    if (event.type === "message_end") {
      const message = event.message as AgentMessage;
      this.session.saveMessage(message);
      if (message.role === "assistant") {
        void this.maybeCompact();
      }
    }
  }

  private async maybeCompact() {
    const messages = this.agent.state.messages.slice();
    const result = await this.session.maybeCompact(messages);
    if (result?.kept) {
      this.agent.replaceMessages(result.kept);
    }
  }
}
