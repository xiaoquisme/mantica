import type { HubOptions } from "./types.js";
import type { ConnectionState } from "../shared/gateway-sdk/types.js";
import { Agent } from "./agent.js";
import { getDeviceId } from "./device.js";
import { GatewayClient } from "../shared/gateway-sdk/client.js";

export class Hub {
  private readonly agents = new Map<string, Agent>();
  private readonly agentSenders = new Map<string, string>();
  private readonly client: GatewayClient;
  readonly url: string;
  readonly path: string;
  readonly deviceId: string;

  /** 当前 Gateway 连接状态 */
  get connectionState(): ConnectionState {
    return this.client.state;
  }

  constructor(url: string, path?: string) {
    this.url = url;
    this.path = path ?? "/ws";
    this.deviceId = getDeviceId();

    this.client = new GatewayClient({
      url: this.url,
      path: this.path,
      deviceId: this.deviceId,
      deviceType: "client",
      autoReconnect: true,
      reconnectDelay: 1000,
    });

    this.client.onStateChange((state) => {
      console.log(`[Hub] Connection state: ${state}`);
    });

    this.client.onRegistered((deviceId) => {
      console.log(`[Hub] Registered as: ${deviceId}`);
    });

    this.client.onError((err) => {
      console.error(`[Hub] Connection error:`, err.message);
    });

    this.client.onMessage((msg) => {
      console.log(`[Hub] Received message: id=${msg.id} from=${msg.from} to=${msg.to} action=${msg.action} payload=${JSON.stringify(msg.payload)}`);
      const payload = msg.payload as { agentId?: string; content?: string } | undefined;
      const agentId = payload?.agentId;
      const content = payload?.content;
      if (!agentId || !content) {
        console.warn(`[Hub] Invalid payload, missing agentId or content`);
        return;
      }
      const agent = this.agents.get(agentId);
      if (agent && !agent.closed) {
        this.agentSenders.set(agentId, msg.from);
        agent.write(content);
      } else {
        console.warn(`[Hub] Agent not found or closed: ${agentId}`);
      }
    });

    this.client.onSendError((err) => {
      console.error(`[Hub] Send error: messageId=${err.messageId} code=${err.code} error=${err.error}`);
    });

    this.client.connect();
  }

  /** 创建新 Agent，或用已有 ID 重建 */
  createAgent(id?: string): Agent {
    if (id) {
      const existing = this.agents.get(id);
      if (existing && !existing.closed) {
        return existing;
      }
    }

    const agent = new Agent(id);
    this.agents.set(agent.id, agent);

    // 内部消费 agent 产出的消息
    void this.consumeAgent(agent);

    console.log(`Agent created: ${agent.id}`);
    return agent;
  }

  /** 内部读取 agent 的输出并通过 Gateway 发送 */
  private async consumeAgent(agent: Agent): Promise<void> {
    for await (const msg of agent.read()) {
      console.log(`[${agent.id}] ${msg.content}`);
      const targetDeviceId = this.agentSenders.get(agent.id);
      if (targetDeviceId) {
        this.client.send(targetDeviceId, "message", {
          agentId: agent.id,
          content: msg.content,
        });
      }
    }
  }

  getAgent(id: string): Agent | undefined {
    return this.agents.get(id);
  }

  listAgents(): string[] {
    return Array.from(this.agents.entries())
      .filter(([, a]) => !a.closed)
      .map(([id]) => id);
  }

  closeAgent(id: string): boolean {
    const agent = this.agents.get(id);
    if (!agent) return false;
    agent.close();
    this.agents.delete(id);
    this.agentSenders.delete(id);
    return true;
  }

  shutdown(): void {
    for (const [id, agent] of this.agents) {
      agent.close();
      this.agents.delete(id);
    }
    this.client.disconnect();
    console.log("Hub shut down");
  }
}
