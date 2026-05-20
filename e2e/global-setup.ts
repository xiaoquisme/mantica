import path from "node:path";
import { writeFileSync, mkdirSync } from "node:fs";
import { TestApiClient } from "./fixtures";

const DEFAULT_E2E_NAME = "E2E User";
const DEFAULT_E2E_EMAIL = "e2e@mantica.ai";
const DEFAULT_E2E_WORKSPACE = "e2e-workspace";

export const SESSION_FILE = path.join(
  process.cwd(),
  "test-results",
  ".e2e-session.json",
);

// Run ONCE before any worker starts. Logs in via the OTP flow (consuming
// one verification code), then writes the JWT to disk so every worker can
// reuse it without re-hitting /auth/send-code (which is rate-limited to 1
// code / 10s / email and marks the code used). Without this, multi-worker
// or multi-file runs cascade into "No verification code found" failures.
export default async function globalSetup() {
  const api = new TestApiClient();
  const data = await api.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
  await api.ensureWorkspace("E2E Workspace", DEFAULT_E2E_WORKSPACE);

  mkdirSync(path.dirname(SESSION_FILE), { recursive: true });
  writeFileSync(
    SESSION_FILE,
    JSON.stringify({
      email: DEFAULT_E2E_EMAIL,
      token: data.token,
      user: data.user ?? null,
    }),
  );
}
