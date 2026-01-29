#!/usr/bin/env node
import { Agent } from "./runner.js";

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

function printUsage() {
  console.log("Usage: pnpm agent:cli [options] <prompt>");
  console.log("       echo \"your prompt\" | pnpm agent:cli");
  console.log("");
  console.log("Options:");
  console.log("  --profile ID     Load agent profile (identity, soul, tools, memory)");
  console.log("  --provider NAME  LLM provider (e.g., openai, anthropic, kimi)");
  console.log("  --model NAME     Model name");
  console.log("  --system TEXT    System prompt (ignored if --profile is set)");
  console.log("  --thinking LEVEL Thinking level");
  console.log("  --cwd DIR        Working directory for commands");
  console.log("  --session ID     Session ID for conversation persistence");
  console.log("  --help, -h       Show this help");
}

function parseArgs(argv: string[]) {
  const args = [...argv];
  const opts: CliOptions = {};
  const promptParts: string[] = [];

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
    if (arg === "--") {
      promptParts.push(...args);
      break;
    }
    promptParts.push(arg);
  }

  return { opts, prompt: promptParts.join(" ") };
}

async function readStdin() {
  if (process.stdin.isTTY) return "";
  return new Promise<string>((resolve, reject) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => (data += chunk));
    process.stdin.on("end", () => resolve(data.trim()));
    process.stdin.on("error", reject);
  });
}

async function main() {
  const { opts, prompt } = parseArgs(process.argv.slice(2));
  if (opts.help) {
    printUsage();
    return;
  }

  const stdinPrompt = await readStdin();
  const finalPrompt = prompt || stdinPrompt;
  if (!finalPrompt) {
    printUsage();
    process.exit(1);
  }

  const agent = new Agent({
    profileId: opts.profile,
    provider: opts.provider,
    model: opts.model,
    systemPrompt: opts.system,
    thinkingLevel: opts.thinking as any,
    cwd: opts.cwd,
    sessionId: opts.session,
  });

  // 如果是新创建的 session，提示用户 sessionId
  if (!opts.session) {
    console.error(`[session: ${agent.sessionId}]`);
  }

  const result = await agent.run(finalPrompt);
  if (result.error) {
    console.error(`Error: ${result.error}`);
    process.exitCode = 1;
  }
}

main().catch((err) => {
  console.error(err?.stack || String(err));
  process.exit(1);
});
