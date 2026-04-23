/**
 * TestApiClient — lightweight API helper for E2E test data setup/teardown.
 *
 * Uses raw fetch so E2E tests have zero build-time coupling to the web app.
 *
 * Also re-exports an extended Playwright `test` that captures a pass screenshot
 * for every scenario. Failure screenshots are produced by Playwright via
 * `use.screenshot: 'only-on-failure'` in playwright.config.ts.
 */

import path from "node:path";
import { test as base, expect } from "@playwright/test";
import pg from "pg";

const SCREENSHOT_DIR = path.join("test-results", "screenshots");

function slugify(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 80) || "scenario";
}

export const test = base.extend({
  page: async ({ page }, use, testInfo) => {
    await use(page);
    if (testInfo.status !== "passed") return;
    const fileSlug = path.basename(testInfo.file).replace(/\.spec\.ts$/, "");
    const filename = `${fileSlug}__${slugify(testInfo.title)}.png`;
    try {
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, filename),
        fullPage: true,
      });
    } catch {
      // Page may already be closed; pass screenshots are best-effort evidence.
    }
  },
});

export { expect };

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? `http://localhost:${process.env.PORT ?? "8080"}`;
const DATABASE_URL = process.env.DATABASE_URL ?? "postgres://multica:multica@localhost:5432/multica?sslmode=disable";

interface TestWorkspace {
  id: string;
  name: string;
  slug: string;
}

interface CachedSession {
  token: string;
  user: { id: string; email: string; name: string } | null;
}

// Module-level token cache, keyed by email. The auth send-code endpoint is
// rate-limited to 1 code per 10 seconds per email and verify-code marks the
// code used. With one login per test (beforeEach), runs of more than two
// tests would otherwise fail at the second test's send-code → can't find an
// unused fresh code. Reusing the JWT (valid for 30 days, see auth.go) keeps
// every test's beforeEach hitting the rate-limited code path at most once.
const sessionCache = new Map<string, CachedSession>();

export class TestApiClient {
  private token: string | null = null;
  private workspaceId: string | null = null;
  private createdIssueIds: string[] = [];
  private createdProjectIds: string[] = [];

  async login(email: string, name: string) {
    const cached = sessionCache.get(email);
    if (cached) {
      this.token = cached.token;
      return cached;
    }

    const data = await this.authenticate(email, name);
    sessionCache.set(email, { token: data.token, user: data.user ?? null });
    return data;
  }

  private async authenticate(email: string, name: string) {
    // Step 1: Send verification code
    const sendRes = await fetch(`${API_BASE}/auth/send-code`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    });
    if (!sendRes.ok) {
      // Rate limited — code already sent recently, read it from DB
      if (sendRes.status !== 429) {
        throw new Error(`send-code failed: ${sendRes.status}`);
      }
    }

    // Step 2: Read code from database
    const client = new pg.Client(DATABASE_URL);
    await client.connect();
    try {
      const result = await client.query(
        "SELECT code FROM verification_code WHERE email = $1 AND used = FALSE AND expires_at > now() ORDER BY created_at DESC LIMIT 1",
        [email]
      );
      if (result.rows.length === 0) {
        throw new Error(`No verification code found for ${email}`);
      }
      const code = result.rows[0].code;

      // Step 3: Verify code to get JWT
      const verifyRes = await fetch(`${API_BASE}/auth/verify-code`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, code }),
      });
      const data = await verifyRes.json();
      this.token = data.token;

      // Update user name if needed
      if (name && data.user?.name !== name) {
        await this.authedFetch("/api/me", {
          method: "PATCH",
          body: JSON.stringify({ name }),
        });
      }

      return data;
    } finally {
      await client.end();
    }
  }

  async getWorkspaces(): Promise<TestWorkspace[]> {
    const res = await this.authedFetch("/api/workspaces");
    return res.json();
  }

  setWorkspaceId(id: string) {
    this.workspaceId = id;
  }

  async ensureWorkspace(name = "E2E Workspace", slug = "e2e-workspace") {
    const workspaces = await this.getWorkspaces();
    const workspace = workspaces.find((item) => item.slug === slug) ?? workspaces[0];
    if (workspace) {
      this.workspaceId = workspace.id;
      return workspace;
    }

    const res = await this.authedFetch("/api/workspaces", {
      method: "POST",
      body: JSON.stringify({ name, slug }),
    });
    if (res.ok) {
      const created = (await res.json()) as TestWorkspace;
      this.workspaceId = created.id;
      return created;
    }

    const refreshed = await this.getWorkspaces();
    const created = refreshed.find((item) => item.slug === slug) ?? refreshed[0];
    if (created) {
      this.workspaceId = created.id;
      return created;
    }

    throw new Error(`Failed to ensure workspace ${slug}: ${res.status} ${res.statusText}`);
  }

  async createIssue(title: string, opts?: Record<string, unknown>) {
    const res = await this.authedFetch("/api/issues", {
      method: "POST",
      body: JSON.stringify({ title, ...opts }),
    });
    const issue = await res.json();
    this.createdIssueIds.push(issue.id);
    return issue;
  }

  async deleteIssue(id: string) {
    await this.authedFetch(`/api/issues/${id}`, { method: "DELETE" });
  }

  async createProject(title: string, opts?: Record<string, unknown>) {
    // status must match the DB project_status_check constraint:
    // planned | in_progress | paused | completed | cancelled
    const res = await this.authedFetch("/api/projects", {
      method: "POST",
      body: JSON.stringify({ title, status: "planned", priority: "none", ...opts }),
    });
    const project = await res.json();
    this.createdProjectIds.push(project.id);
    return project;
  }

  async deleteProject(id: string) {
    await this.authedFetch(`/api/projects/${id}`, { method: "DELETE" });
  }

  /** Clean up all issues and projects created during this test. */
  async cleanup() {
    for (const id of this.createdIssueIds) {
      try {
        await this.deleteIssue(id);
      } catch {
        /* ignore — may already be deleted */
      }
    }
    this.createdIssueIds = [];
    for (const id of this.createdProjectIds) {
      try {
        await this.deleteProject(id);
      } catch {
        /* ignore — may already be deleted */
      }
    }
    this.createdProjectIds = [];
  }

  getToken() {
    return this.token;
  }

  private async authedFetch(path: string, init?: RequestInit) {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((init?.headers as Record<string, string>) ?? {}),
    };
    if (this.token) headers["Authorization"] = `Bearer ${this.token}`;
    if (this.workspaceId) headers["X-Workspace-ID"] = this.workspaceId;
    return fetch(`${API_BASE}${path}`, { ...init, headers });
  }
}
