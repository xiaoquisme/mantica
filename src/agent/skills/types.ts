/**
 * Skills Module Types
 *
 * Type definitions for the skills system
 */

/**
 * Skill metadata for eligibility and display
 */
export interface SkillMetadata {
  /** Emoji for display (e.g., "📝") */
  emoji?: string | undefined;
  /** Required environment variables */
  requiresEnv?: string[] | undefined;
  /** Required binaries in PATH */
  requiresBinaries?: string[] | undefined;
  /** Supported platforms (darwin, linux, win32) */
  platforms?: string[] | undefined;
  /** Skill tags for categorization */
  tags?: string[] | undefined;
}

/**
 * SKILL.md frontmatter schema
 */
export interface SkillFrontmatter {
  /** Skill name (required) */
  name: string;
  /** Human-readable description */
  description?: string | undefined;
  /** Skill version */
  version?: string | undefined;
  /** Author information */
  author?: string | undefined;
  /** Homepage/documentation URL */
  homepage?: string | undefined;
  /** Skill-specific metadata */
  metadata?: SkillMetadata | undefined;
}

/**
 * Skill source type with precedence (lower value = lower priority)
 */
export type SkillSource = "bundled" | "profile";

/**
 * Skill source precedence values
 */
export const SKILL_SOURCE_PRECEDENCE: Record<SkillSource, number> = {
  bundled: 0,
  profile: 1,
};

/**
 * Parsed skill entry
 */
export interface Skill {
  /** Unique skill identifier (directory name) */
  id: string;
  /** Parsed frontmatter */
  frontmatter: SkillFrontmatter;
  /** Skill instructions (markdown body after frontmatter) */
  instructions: string;
  /** Source type */
  source: SkillSource;
  /** Full path to SKILL.md */
  filePath: string;
}

/**
 * Skill Manager options
 */
export interface SkillManagerOptions {
  /** Agent profile ID (for profile-specific skills) */
  profileId?: string | undefined;
  /** Profile base directory, defaults to ~/.super-multica/agent-profiles */
  profileBaseDir?: string | undefined;
  /** Additional directories to search for skills */
  extraDirs?: string[] | undefined;
  /** Platform override (for testing) */
  platform?: NodeJS.Platform | undefined;
}

/**
 * Skill eligibility check result
 */
export interface EligibilityResult {
  /** Whether the skill is eligible */
  eligible: boolean;
  /** Reasons for ineligibility */
  reasons?: string[] | undefined;
}

/**
 * Filename constant for skill definition file
 */
export const SKILL_FILE = "SKILL.md";
