/**
 * SKILL.md Parser
 *
 * Parse skill files with YAML frontmatter and markdown body
 */

import { readFileSync } from "node:fs";
import { parse as parseYaml } from "yaml";
import type { Skill, SkillFrontmatter, SkillSource } from "./types.js";

/**
 * Parse YAML frontmatter from markdown content
 *
 * @param content - Raw markdown content
 * @returns Tuple of [frontmatter object or null, body content]
 */
export function parseFrontmatter(content: string): [Record<string, unknown> | null, string] {
  // Match frontmatter between --- delimiters at the start
  const match = content.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n([\s\S]*)$/);
  if (!match) {
    return [null, content.trim()];
  }

  const frontmatterRaw = match[1] ?? "";
  const body = match[2] ?? "";

  if (!frontmatterRaw) {
    return [null, content.trim()];
  }

  try {
    const frontmatter = parseYaml(frontmatterRaw) as Record<string, unknown>;
    return [frontmatter, body.trim()];
  } catch {
    // Invalid YAML, return null frontmatter
    return [null, content.trim()];
  }
}

/**
 * Validate and coerce frontmatter to SkillFrontmatter type
 *
 * @param raw - Raw parsed frontmatter
 * @returns Validated SkillFrontmatter or null if invalid
 */
function validateFrontmatter(raw: Record<string, unknown>): SkillFrontmatter | null {
  // Name is required
  if (typeof raw.name !== "string" || raw.name.trim() === "") {
    return null;
  }

  const frontmatter: SkillFrontmatter = {
    name: raw.name.trim(),
  };

  if (typeof raw.description === "string") {
    frontmatter.description = raw.description;
  }

  if (typeof raw.version === "string") {
    frontmatter.version = raw.version;
  }

  if (typeof raw.author === "string") {
    frontmatter.author = raw.author;
  }

  if (typeof raw.homepage === "string") {
    frontmatter.homepage = raw.homepage;
  }

  // Parse metadata if present
  if (typeof raw.metadata === "object" && raw.metadata !== null) {
    const meta = raw.metadata as Record<string, unknown>;
    frontmatter.metadata = {
      emoji: typeof meta.emoji === "string" ? meta.emoji : undefined,
      requiresEnv: Array.isArray(meta.requiresEnv)
        ? meta.requiresEnv.filter((v): v is string => typeof v === "string")
        : undefined,
      requiresBinaries: Array.isArray(meta.requiresBinaries)
        ? meta.requiresBinaries.filter((v): v is string => typeof v === "string")
        : undefined,
      platforms: Array.isArray(meta.platforms)
        ? meta.platforms.filter((v): v is string => typeof v === "string")
        : undefined,
      tags: Array.isArray(meta.tags)
        ? meta.tags.filter((v): v is string => typeof v === "string")
        : undefined,
    };
  }

  return frontmatter;
}

/**
 * Parse a SKILL.md file into a Skill object
 *
 * @param filePath - Full path to SKILL.md file
 * @param skillId - Unique identifier for the skill
 * @param source - Source type of the skill
 * @returns Parsed Skill or null if invalid
 */
export function parseSkillFile(
  filePath: string,
  skillId: string,
  source: SkillSource,
): Skill | null {
  try {
    const content = readFileSync(filePath, "utf-8");
    const [rawFrontmatter, instructions] = parseFrontmatter(content);

    if (!rawFrontmatter) {
      return null;
    }

    const frontmatter = validateFrontmatter(rawFrontmatter);
    if (!frontmatter) {
      return null;
    }

    return {
      id: skillId,
      frontmatter,
      instructions,
      source,
      filePath,
    };
  } catch {
    // File read error or other issues
    return null;
  }
}
