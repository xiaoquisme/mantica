import { v7 as uuidv7 } from "uuid";
import { Channel } from "./channel.js";
import type { Message } from "./types.js";

/**
 * Mock Agent — 本地回环实现，用于测试。
 * write() 将消息放入 channel，read() 从 channel 读取。
 */
export class Agent {
  readonly id: string;
  private readonly channel = new Channel<Message>();
  private _closed = false;

  constructor(id?: string) {
    this.id = id ?? uuidv7();
  }

  get closed(): boolean {
    return this._closed;
  }

  /** 写入消息到 agent（非阻塞） */
  write(content: string): void {
    if (this._closed) {
      throw new Error("Agent is closed");
    }
    this.channel.send({
      id: uuidv7(),
      content: `[mock-agent:${this.id}] echo: ${content}`,
    });
  }

  /** 持续读取消息流 */
  read(): AsyncIterable<Message> {
    return this.channel;
  }

  /** 关闭 agent，停止所有读取 */
  close(): void {
    if (this._closed) return;
    this._closed = true;
    this.channel.close();
  }
}
