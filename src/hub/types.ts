export interface Message {
  readonly id: string;
  readonly content: string;
}

export interface HubOptions {
  /** 远端 Gateway WebSocket 地址，如 "http://localhost:3000" */
  url: string;
  /** WebSocket 路径，默认 "/ws" */
  path?: string | undefined;
}
