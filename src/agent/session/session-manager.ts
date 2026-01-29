import type { AgentMessage } from "@mariozechner/pi-agent-core";
import type { Model } from "@mariozechner/pi-ai";
import type { SessionEntry, SessionMeta } from "./types.js";
import { appendEntry, readEntries, writeEntries } from "./storage.js";
import { compactMessages, compactMessagesAsync } from "./compaction.js";

export type SessionManagerOptions = {
  sessionId: string;
  baseDir?: string | undefined;

  // Compaction 模式配置
  /** Compaction 模式: "count" 使用消息计数, "tokens" 使用 token 感知, "summary" 使用 LLM 摘要 */
  compactionMode?: "count" | "tokens" | "summary" | undefined;

  // Count 模式参数
  maxMessages?: number | undefined;
  keepLast?: number | undefined;

  // Token 模式参数
  /** Context window token 数 */
  contextWindowTokens?: number | undefined;
  /** 系统提示词（用于计算可用 token） */
  systemPrompt?: string | undefined;
  /** 预留给响应的 token 数 */
  reserveTokens?: number | undefined;
  /** Compaction 目标利用率 (0-1) */
  targetRatio?: number | undefined;
  /** 最小保留消息数 */
  minKeepMessages?: number | undefined;

  // Summary 模式参数
  /** LLM Model（用于生成摘要） */
  model?: Model<any> | undefined;
  /** API Key */
  apiKey?: string | undefined;
  /** 自定义摘要指令 */
  customInstructions?: string | undefined;
};

export class SessionManager {
  private readonly sessionId: string;
  private readonly baseDir: string | undefined;
  private readonly compactionMode: "count" | "tokens" | "summary";
  // Count 模式
  private readonly maxMessages: number;
  private readonly keepLast: number;
  // Token 模式
  private readonly contextWindowTokens: number;
  private systemPrompt: string | undefined;
  private readonly reserveTokens: number;
  private readonly targetRatio: number;
  private readonly minKeepMessages: number;
  // Summary 模式
  private model: Model<any> | undefined;
  private apiKey: string | undefined;
  private readonly customInstructions: string | undefined;
  private previousSummary: string | undefined;

  private queue: Promise<void> = Promise.resolve();
  private meta: SessionMeta | undefined;

  constructor(options: SessionManagerOptions) {
    this.sessionId = options.sessionId;
    this.baseDir = options.baseDir;

    // Compaction 模式
    this.compactionMode = options.compactionMode ?? "count";

    // Count 模式参数
    this.maxMessages = options.maxMessages ?? 80;
    this.keepLast = options.keepLast ?? 60;

    // Token 模式参数
    this.contextWindowTokens = options.contextWindowTokens ?? 200_000;
    this.systemPrompt = options.systemPrompt;
    this.reserveTokens = options.reserveTokens ?? 1024;
    this.targetRatio = options.targetRatio ?? 0.5;
    this.minKeepMessages = options.minKeepMessages ?? 10;

    // Summary 模式参数
    this.model = options.model;
    this.apiKey = options.apiKey;
    this.customInstructions = options.customInstructions;

    this.meta = this.loadMeta();
  }

  /**
   * 更新系统提示词（用于 token 模式计算）
   */
  setSystemPrompt(systemPrompt: string | undefined) {
    this.systemPrompt = systemPrompt;
  }

  /**
   * 获取当前 context window token 数
   */
  getContextWindowTokens(): number {
    return this.contextWindowTokens;
  }

  /**
   * 设置 LLM Model（用于 summary 模式）
   */
  setModel(model: Model<any> | undefined) {
    this.model = model;
  }

  /**
   * 设置 API Key（用于 summary 模式）
   */
  setApiKey(apiKey: string | undefined) {
    this.apiKey = apiKey;
  }

  /**
   * 获取当前 compaction 模式
   */
  getCompactionMode(): "count" | "tokens" | "summary" {
    return this.compactionMode;
  }

  loadEntries(): SessionEntry[] {
    return readEntries(this.sessionId, { baseDir: this.baseDir });
  }

  loadMessages(): AgentMessage[] {
    const entries = this.loadEntries();
    return entries
      .filter((entry) => entry.type === "message")
      .map((entry) => entry.message);
  }

  loadMeta(): SessionMeta | undefined {
    const entries = this.loadEntries();
    let meta: SessionMeta | undefined;
    for (const entry of entries) {
      if (entry.type === "meta") {
        meta = entry.meta;
      }
    }
    return meta;
  }

  getMeta(): SessionMeta | undefined {
    return this.meta;
  }

  saveMeta(meta: SessionMeta) {
    this.meta = meta;
    void this.enqueue(() =>
      appendEntry(
        this.sessionId,
        { type: "meta", meta, timestamp: Date.now() },
        { baseDir: this.baseDir },
      ),
    );
  }

  saveMessage(message: AgentMessage) {
    void this.enqueue(() =>
      appendEntry(
        this.sessionId,
        { type: "message", message, timestamp: Date.now() },
        { baseDir: this.baseDir },
      ),
    );
  }

  async maybeCompact(messages: AgentMessage[]) {
    let result;

    if (this.compactionMode === "summary") {
      // Summary 模式需要 model 和 apiKey
      if (!this.model || !this.apiKey) {
        // 降级到 tokens 模式
        result = compactMessages(messages, {
          mode: "tokens",
          contextWindowTokens: this.contextWindowTokens,
          systemPrompt: this.systemPrompt,
          reserveTokens: this.reserveTokens,
          targetRatio: this.targetRatio,
          minKeepMessages: this.minKeepMessages,
        });
      } else {
        result = await compactMessagesAsync(messages, {
          mode: "summary",
          model: this.model,
          apiKey: this.apiKey,
          contextWindowTokens: this.contextWindowTokens,
          systemPrompt: this.systemPrompt,
          reserveTokens: this.reserveTokens,
          targetRatio: this.targetRatio,
          minKeepMessages: this.minKeepMessages,
          customInstructions: this.customInstructions,
          previousSummary: this.previousSummary,
        });

        // 保存摘要用于下次增量更新
        if (result?.summary) {
          this.previousSummary = result.summary;
        }
      }
    } else {
      result = compactMessages(messages, {
        mode: this.compactionMode,
        // Count 模式参数
        maxMessages: this.maxMessages,
        keepLast: this.keepLast,
        // Token 模式参数
        contextWindowTokens: this.contextWindowTokens,
        systemPrompt: this.systemPrompt,
        reserveTokens: this.reserveTokens,
        targetRatio: this.targetRatio,
        minKeepMessages: this.minKeepMessages,
      });
    }

    if (!result) return null;

    const entries: SessionEntry[] = [];
    if (this.meta) {
      entries.push({ type: "meta", meta: this.meta, timestamp: Date.now() });
    }
    for (const message of result.kept) {
      entries.push({ type: "message", message, timestamp: Date.now() });
    }
    entries.push({
      type: "compaction",
      removed: result.removedCount,
      kept: result.kept.length,
      timestamp: Date.now(),
      // Token/Summary 模式下的额外信息
      tokensRemoved: result.tokensRemoved,
      tokensKept: result.tokensKept,
      summary: result.summary,
      reason: result.reason,
    });

    await this.enqueue(() =>
      writeEntries(this.sessionId, entries, { baseDir: this.baseDir }),
    );
    return result;
  }

  private enqueue(task: () => Promise<void>) {
    this.queue = this.queue.then(task, task);
    return this.queue;
  }
}
