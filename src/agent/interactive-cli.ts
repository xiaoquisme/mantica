#!/usr/bin/env node
import * as readline from "readline";
import { Agent } from "./runner.js";
import type { AgentOptions } from "./types.js";

type CliOptions = {
  profile?: string | undefined;
  provider?: string | undefined;
  model?: string | undefined;
  system?: string | undefined;
  thinking?: string | undefined;
  cwd?: string | undefined;
  session?: string | undefined;
  help?: boolean | undefined;
};

const COMMANDS = {
  help: "Show this help message",
  exit: "Exit the CLI (aliases: quit, q)",
  clear: "Clear the current session and start fresh",
  session: "Show current session ID",
  new: "Start a new session",
  multiline: "Toggle multi-line input mode (end with a line containing only '.')",
};

function printUsage() {
  console.log("Usage: pnpm agent:interactive [options]");
  console.log("");
  console.log("Options:");
  console.log("  --profile ID     Load agent profile (identity, soul, tools, memory)");
  console.log("  --provider NAME  LLM provider (e.g., openai, anthropic, kimi)");
  console.log("  --model NAME     Model name");
  console.log("  --system TEXT    System prompt (ignored if --profile is set)");
  console.log("  --thinking LEVEL Thinking level");
  console.log("  --cwd DIR        Working directory for commands");
  console.log("  --session ID     Session ID to resume");
  console.log("  --help, -h       Show this help");
  console.log("");
  console.log("Commands (use during interaction):");
  for (const [cmd, desc] of Object.entries(COMMANDS)) {
    console.log(`  /${cmd.padEnd(12)} ${desc}`);
  }
}

function parseArgs(argv: string[]) {
  const args = [...argv];
  const opts: CliOptions = {};

  while (args.length > 0) {
    const arg = args.shift();
    if (!arg) break;
    if (arg === "--help" || arg === "-h") {
      opts.help = true;
      break;
    }
    if (arg === "--profile") {
      opts.profile = args.shift();
      continue;
    }
    if (arg === "--provider") {
      opts.provider = args.shift();
      continue;
    }
    if (arg === "--model") {
      opts.model = args.shift();
      continue;
    }
    if (arg === "--system") {
      opts.system = args.shift();
      continue;
    }
    if (arg === "--thinking") {
      opts.thinking = args.shift();
      continue;
    }
    if (arg === "--cwd") {
      opts.cwd = args.shift();
      continue;
    }
    if (arg === "--session") {
      opts.session = args.shift();
      continue;
    }
  }

  return opts;
}

function printWelcome(sessionId: string) {
  console.log("╭─────────────────────────────────────────╮");
  console.log("│     Super Multica Interactive CLI       │");
  console.log("╰─────────────────────────────────────────╯");
  console.log(`Session: ${sessionId}`);
  console.log("Type /help for available commands, /exit to quit.");
  console.log("");
}

function printHelp() {
  console.log("\nAvailable commands:");
  for (const [cmd, desc] of Object.entries(COMMANDS)) {
    console.log(`  /${cmd.padEnd(12)} ${desc}`);
  }
  console.log("\nJust type your message and press Enter to chat with the agent.");
  console.log("");
}

class InteractiveCLI {
  private agent: Agent;
  private opts: CliOptions;
  private rl: readline.Interface;
  private multilineMode = false;
  private multilineBuffer: string[] = [];
  private running = true;

  constructor(opts: CliOptions) {
    this.opts = opts;
    this.agent = this.createAgent(opts.session);

    this.rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout,
      terminal: true,
    });

    this.rl.on("close", () => {
      this.running = false;
      console.log("\nGoodbye!");
      process.exit(0);
    });
  }

  private createAgent(sessionId?: string): Agent {
    return new Agent({
      profileId: this.opts.profile,
      provider: this.opts.provider,
      model: this.opts.model,
      systemPrompt: this.opts.system,
      thinkingLevel: this.opts.thinking as AgentOptions["thinkingLevel"],
      cwd: this.opts.cwd,
      sessionId,
    });
  }

  private prompt(): string {
    if (this.multilineMode) {
      return this.multilineBuffer.length === 0 ? ">>> " : "... ";
    }
    return "You: ";
  }

  async run() {
    printWelcome(this.agent.sessionId);
    await this.loop();
  }

  private async loop() {
    while (this.running) {
      const input = await this.readline(this.prompt());
      if (input === null) break;

      if (this.multilineMode) {
        if (input === ".") {
          // End of multiline input
          const fullInput = this.multilineBuffer.join("\n");
          this.multilineBuffer = [];
          this.multilineMode = false;
          if (fullInput.trim()) {
            await this.handleInput(fullInput);
          }
        } else {
          this.multilineBuffer.push(input);
        }
        continue;
      }

      const trimmed = input.trim();
      if (!trimmed) continue;

      if (trimmed.startsWith("/")) {
        const handled = await this.handleCommand(trimmed);
        if (!handled) {
          await this.handleInput(trimmed);
        }
      } else {
        await this.handleInput(trimmed);
      }
    }
  }

  private readline(prompt: string): Promise<string | null> {
    return new Promise((resolve) => {
      this.rl.question(prompt, (answer) => {
        resolve(answer);
      });
    });
  }

  private async handleCommand(input: string): Promise<boolean> {
    const cmd = input.slice(1).toLowerCase().split(/\s+/)[0];

    switch (cmd) {
      case "help":
        printHelp();
        return true;

      case "exit":
      case "quit":
      case "q":
        console.log("Goodbye!");
        this.running = false;
        this.rl.close();
        process.exit(0);
        return true;

      case "clear":
        this.agent = this.createAgent();
        console.log(`Session cleared. New session: ${this.agent.sessionId}\n`);
        return true;

      case "session":
        console.log(`Current session: ${this.agent.sessionId}\n`);
        return true;

      case "new":
        this.agent = this.createAgent();
        console.log(`Started new session: ${this.agent.sessionId}\n`);
        return true;

      case "multiline":
        this.multilineMode = !this.multilineMode;
        if (this.multilineMode) {
          console.log("Multi-line mode enabled. End input with a line containing only '.'");
          this.multilineBuffer = [];
        } else {
          console.log("Multi-line mode disabled.");
          this.multilineBuffer = [];
        }
        return true;

      default:
        // Unknown command - let the agent handle it
        return false;
    }
  }

  private async handleInput(input: string) {
    try {
      console.log(""); // Add spacing before response
      const result = await this.agent.run(input);
      if (result.error) {
        console.error(`\nError: ${result.error}`);
      }
      console.log(""); // Add spacing after response
    } catch (err) {
      console.error(`\nError: ${err instanceof Error ? err.message : String(err)}`);
      console.log("");
    }
  }
}

async function main() {
  const opts = parseArgs(process.argv.slice(2));

  if (opts.help) {
    printUsage();
    return;
  }

  // Check if running in a TTY
  if (!process.stdin.isTTY) {
    console.error("Error: Interactive CLI requires a TTY. Use agent:cli for non-interactive mode.");
    process.exit(1);
  }

  const cli = new InteractiveCLI(opts);
  await cli.run();
}

main().catch((err) => {
  console.error(err?.stack || String(err));
  process.exit(1);
});
