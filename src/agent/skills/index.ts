/**
 * Skills Module
 *
 * Manages skill loading, eligibility filtering, and system prompt generation
 */

import type { Skill, SkillManagerOptions } from "./types.js";
import { loadAllSkills, getBundledSkillsDir, getProfileSkillsDir } from "./loader.js";
import { filterEligibleSkills, checkEligibility } from "./eligibility.js";

// Re-export types and utilities
export type {
  Skill,
  SkillFrontmatter,
  SkillMetadata,
  SkillSource,
  SkillManagerOptions,
  EligibilityResult,
} from "./types.js";

export { SKILL_FILE, SKILL_SOURCE_PRECEDENCE } from "./types.js";
export { checkEligibility, filterEligibleSkills } from "./eligibility.js";
export { parseFrontmatter, parseSkillFile } from "./parser.js";
export { loadAllSkills, getBundledSkillsDir, getProfileSkillsDir } from "./loader.js";

/**
 * SkillManager - Loads and manages skills
 *
 * Provides access to skills from multiple sources with precedence handling
 * and eligibility filtering.
 */
export class SkillManager {
  private readonly options: SkillManagerOptions;
  private skills: Map<string, Skill> | undefined;
  private eligibleSkills: Map<string, Skill> | undefined;

  constructor(options: SkillManagerOptions = {}) {
    this.options = options;
  }

  /**
   * Ensure skills are loaded (lazy loading)
   */
  private ensureLoaded(): void {
    if (this.skills) return;
    this.skills = loadAllSkills(this.options);
    this.eligibleSkills = filterEligibleSkills(this.skills, this.options.platform);
  }

  /**
   * Get all loaded skills (including ineligible)
   */
  getAllSkills(): Map<string, Skill> {
    this.ensureLoaded();
    return this.skills!;
  }

  /**
   * Get only eligible skills
   */
  getEligibleSkills(): Map<string, Skill> {
    this.ensureLoaded();
    return this.eligibleSkills!;
  }

  /**
   * Get a specific skill by ID (only from eligible skills)
   *
   * @param skillId - Skill identifier
   * @returns Skill or undefined if not found or ineligible
   */
  getSkill(skillId: string): Skill | undefined {
    this.ensureLoaded();
    return this.eligibleSkills!.get(skillId);
  }

  /**
   * Get skill by ID from all skills (including ineligible)
   *
   * @param skillId - Skill identifier
   * @returns Skill or undefined if not found
   */
  getSkillFromAll(skillId: string): Skill | undefined {
    this.ensureLoaded();
    return this.skills!.get(skillId);
  }

  /**
   * Reload skills from disk
   * Clears cache and reloads on next access
   */
  reload(): void {
    this.skills = undefined;
    this.eligibleSkills = undefined;
  }

  /**
   * Build skills section for system prompt
   *
   * Generates formatted documentation of all eligible skills
   * for inclusion in the agent's system prompt.
   *
   * @returns Formatted skill documentation or empty string if no skills
   */
  buildSkillsPrompt(): string {
    this.ensureLoaded();

    if (this.eligibleSkills!.size === 0) {
      return "";
    }

    const parts: string[] = [];
    parts.push("# Available Skills\n");
    parts.push("You have access to the following skills:\n");

    for (const [id, skill] of this.eligibleSkills!) {
      const emoji = skill.frontmatter.metadata?.emoji ?? "🔧";
      const name = skill.frontmatter.name;
      const desc = skill.frontmatter.description ?? "No description provided";

      parts.push(`## ${emoji} ${name} (${id})`);
      parts.push(`${desc}\n`);

      // Include full instructions
      if (skill.instructions) {
        parts.push(skill.instructions);
        parts.push("");
      }
    }

    return parts.join("\n");
  }

  /**
   * Get skill instructions for a specific skill
   *
   * @param skillId - Skill identifier
   * @returns Instructions markdown or undefined if not found
   */
  getSkillInstructions(skillId: string): string | undefined {
    const skill = this.getSkill(skillId);
    return skill?.instructions;
  }

  /**
   * List skill IDs with their display info
   *
   * @returns Array of skill info for display
   */
  listSkills(): Array<{ id: string; name: string; emoji: string; description: string }> {
    this.ensureLoaded();

    const result: Array<{ id: string; name: string; emoji: string; description: string }> = [];

    for (const [id, skill] of this.eligibleSkills!) {
      result.push({
        id,
        name: skill.frontmatter.name,
        emoji: skill.frontmatter.metadata?.emoji ?? "🔧",
        description: skill.frontmatter.description ?? "No description",
      });
    }

    return result;
  }
}
