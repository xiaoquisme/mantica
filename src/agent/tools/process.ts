import { spawn, type ChildProcess } from "child_process";
import { Type } from "@sinclair/typebox";
import type { AgentTool } from "@mariozechner/pi-agent-core";
import { v7 as uuidv7 } from "uuid";

const ProcessSchema = Type.Object({
  action: Type.String({ description: "Action: start | status | stop | output." }),
  id: Type.Optional(Type.String({ description: "Process id for status/stop/output." })),
  command: Type.Optional(Type.String({ description: "Command to run for start." })),
  cwd: Type.Optional(Type.String({ description: "Working directory." })),
});

const MAX_OUTPUT_BUFFER = 64 * 1024; // 64KB per process

type ProcessEntry = {
  id: string;
  command: string;
  cwd?: string | undefined;
  child: ChildProcess;
  exitCode: number | null;
  startedAt: number;
  outputBuffer: string[];
  outputSize: number;
};

const PROCESS_REGISTRY = new Map<string, ProcessEntry>();

export type ProcessResult = {
  id?: string | undefined;
  running?: boolean | undefined;
  exitCode?: number | null | undefined;
  message?: string | undefined;
  output?: string | undefined;
};

export function createProcessTool(defaultCwd?: string): AgentTool<typeof ProcessSchema, ProcessResult> {
  return {
    name: "process",
    label: "Process",
    description: "Manage long-running background processes like servers, watchers, or daemons. Actions: 'start' to launch (returns immediately with process id), 'status' to check if running, 'output' to read stdout/stderr, 'stop' to terminate. Use this for servers (e.g., python server.py, npm run dev) instead of 'exec'.",
    parameters: ProcessSchema,
    execute: async (_toolCallId, params, signal) => {
      const action = String(params.action ?? "").toLowerCase();
      if (!action) {
        throw new Error("Missing action");
      }

      if (action === "start") {
        const command = String(params.command ?? "");
        if (!command) throw new Error("Missing command");
        const id = params.id ? String(params.id) : uuidv7();
        if (PROCESS_REGISTRY.has(id)) {
          throw new Error(`Process already exists: ${id}`);
        }

        // 使用 Promise 等待进程启动或失败
        const result = await new Promise<{ success: boolean; error?: string }>((resolve) => {
          const child = spawn(command, {
            shell: true,
            cwd: params.cwd || defaultCwd,
            stdio: ["ignore", "pipe", "pipe"],
            detached: true,
          });

          let resolved = false;

          // 处理 spawn 错误（如 shell 不存在）
          child.on("error", (err) => {
            if (!resolved) {
              resolved = true;
              resolve({ success: false, error: err.message });
            }
          });

          // 进程启动成功后注册到 registry
          child.on("spawn", () => {
            if (!resolved) {
              resolved = true;
              const entry: ProcessEntry = {
                id,
                command,
                cwd: params.cwd || defaultCwd,
                child,
                exitCode: null,
                startedAt: Date.now(),
                outputBuffer: [],
                outputSize: 0,
              };
              PROCESS_REGISTRY.set(id, entry);

              // 收集输出到缓冲区
              const collectOutput = (data: Buffer) => {
                const text = data.toString("utf8");
                if (entry.outputSize + text.length > MAX_OUTPUT_BUFFER) {
                  // 超出限制时，移除旧的输出
                  while (entry.outputBuffer.length > 0 && entry.outputSize + text.length > MAX_OUTPUT_BUFFER) {
                    const removed = entry.outputBuffer.shift();
                    if (removed) entry.outputSize -= removed.length;
                  }
                }
                entry.outputBuffer.push(text);
                entry.outputSize += text.length;
              };

              child.stdout?.on("data", collectOutput);
              child.stderr?.on("data", collectOutput);

              child.on("close", (code) => {
                entry.exitCode = code;
              });

              if (signal) {
                signal.addEventListener("abort", () => {
                  child.kill("SIGTERM");
                });
              }

              resolve({ success: true });
            }
          });
        });

        if (!result.success) {
          return {
            content: [{ type: "text", text: `Failed to start process: ${result.error}` }],
            details: { id, running: false, message: result.error },
          };
        }

        return {
          content: [{ type: "text", text: `Started process ${id}` }],
          details: { id, running: true },
        };
      }

      if (action === "status") {
        const id = String(params.id ?? "");
        const entry = PROCESS_REGISTRY.get(id);
        if (!entry) {
          return {
            content: [{ type: "text", text: `Process not found: ${id}` }],
            details: { id, running: false },
          };
        }
        const running = entry.exitCode === null;
        return {
          content: [{ type: "text", text: running ? `Process running: ${id}` : `Process exited: ${id}` }],
          details: { id, running, exitCode: entry.exitCode },
        };
      }

      if (action === "stop") {
        const id = String(params.id ?? "");
        const entry = PROCESS_REGISTRY.get(id);
        if (!entry) {
          return {
            content: [{ type: "text", text: `Process not found: ${id}` }],
            details: { id, running: false },
          };
        }
        entry.child.kill("SIGTERM");
        return {
          content: [{ type: "text", text: `Stopped process ${id}` }],
          details: { id, running: false },
        };
      }

      if (action === "output") {
        const id = String(params.id ?? "");
        const entry = PROCESS_REGISTRY.get(id);
        if (!entry) {
          return {
            content: [{ type: "text", text: `Process not found: ${id}` }],
            details: { id, running: false },
          };
        }
        const output = entry.outputBuffer.join("");
        const running = entry.exitCode === null;
        return {
          content: [{ type: "text", text: output || "(no output)" }],
          details: { id, running, exitCode: entry.exitCode, output },
        };
      }

      throw new Error(`Unknown action: ${action}`);
    },
  };
}
