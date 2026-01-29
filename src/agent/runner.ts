import { Agent as PiAgentCore, type AgentEvent, type AgentMessage } from "@mariozechner/pi-agent-core";
import { v7 as uuidv7 } from "uuid";
import type { AgentOptions, AgentRunResult } from "./types.js";
import { createAgentOutput } from "./output.js";
import { resolveModel, resolveTools } from "./tools.js";
import { SessionManager } from "./session/session-manager.js";
import { ProfileManager } from "./profile/index.js";

export class Agent {
  private readonly agent: PiAgentCore;
  private readonly output;
  private readonly session: SessionManager;
  private readonly profile?: ProfileManager;

  /** 当前会话 ID */
  readonly sessionId: string;

  constructor(options: AgentOptions = {}) {
    const stdout = options.logger?.stdout ?? process.stdout;
    const stderr = options.logger?.stderr ?? process.stderr;
    this.output = createAgentOutput({ stdout, stderr });

    this.agent = new PiAgentCore();

    // 加载 Agent Profile（如果指定了 profileId）
    if (options.profileId) {
      this.profile = new ProfileManager({
        profileId: options.profileId,
        baseDir: options.profileBaseDir,
      });
      const systemPrompt = this.profile.buildSystemPrompt();
      if (systemPrompt) {
        this.agent.setSystemPrompt(systemPrompt);
      }
    } else if (options.systemPrompt) {
      // 直接使用传入的 systemPrompt
      this.agent.setSystemPrompt(options.systemPrompt);
    }

    this.sessionId = options.sessionId ?? uuidv7();
    this.session = new SessionManager({ sessionId: this.sessionId });
    const storedMeta = this.session.getMeta();
    if (!options.thinkingLevel && storedMeta?.thinkingLevel) {
      this.agent.setThinkingLevel(storedMeta.thinkingLevel as any);
    } else if (options.thinkingLevel) {
      this.agent.setThinkingLevel(options.thinkingLevel);
    }

    const model = options.provider && options.model ? resolveModel(options) : resolveModel({
      ...options,
      provider: storedMeta?.provider,
      model: storedMeta?.model,
    });
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
