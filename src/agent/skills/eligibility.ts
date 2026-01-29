/**
 * Skill Eligibility Checker
 *
 * Filter skills based on platform, binaries, and environment requirements
 */

import { execSync } from "node:child_process";
import type { Skill, EligibilityResult } from "./types.js";

/**
 * Check if a binary exists in PATH
 *
 * @param binary - Binary name to check
 * @returns True if binary exists
 */
function binaryExists(binary: string): boolean {
  try {
    // Use 'which' on Unix, 'where' on Windows
    const cmd = process.platform === "win32" ? `where ${binary}` : `which ${binary}`;
    execSync(cmd, { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

/**
 * Check if an environment variable is set
 *
 * @param envVar - Environment variable name
 * @returns True if set (even if empty string)
 */
function envExists(envVar: string): boolean {
  return envVar in process.env;
}

/**
 * Check if a skill is eligible based on its requirements
 *
 * @param skill - Skill to check
 * @param platform - Platform to check against (defaults to current)
 * @returns Eligibility result with reasons if ineligible
 */
export function checkEligibility(
  skill: Skill,
  platform: NodeJS.Platform = process.platform,
): EligibilityResult {
  const reasons: string[] = [];
  const metadata = skill.frontmatter.metadata;

  // No metadata means no requirements
  if (!metadata) {
    return { eligible: true };
  }

  // Platform check
  if (metadata.platforms && metadata.platforms.length > 0) {
    if (!metadata.platforms.includes(platform)) {
      reasons.push(
        `Platform '${platform}' not supported (requires: ${metadata.platforms.join(", ")})`,
      );
    }
  }

  // Binary requirements check
  if (metadata.requiresBinaries && metadata.requiresBinaries.length > 0) {
    for (const binary of metadata.requiresBinaries) {
      if (!binaryExists(binary)) {
        reasons.push(`Required binary not found: ${binary}`);
      }
    }
  }

  // Environment variable check
  if (metadata.requiresEnv && metadata.requiresEnv.length > 0) {
    for (const envVar of metadata.requiresEnv) {
      if (!envExists(envVar)) {
        reasons.push(`Required environment variable not set: ${envVar}`);
      }
    }
  }

  return {
    eligible: reasons.length === 0,
    reasons: reasons.length > 0 ? reasons : undefined,
  };
}

/**
 * Filter skills by eligibility
 *
 * @param skills - Map of skills to filter
 * @param platform - Platform to check against
 * @returns Map containing only eligible skills
 */
export function filterEligibleSkills(
  skills: Map<string, Skill>,
  platform?: NodeJS.Platform,
): Map<string, Skill> {
  const eligible = new Map<string, Skill>();

  for (const [id, skill] of skills) {
    const result = checkEligibility(skill, platform);
    if (result.eligible) {
      eligible.set(id, skill);
    }
  }

  return eligible;
}
