
export interface SkillQualityInfo {
  id: string;
  name: string;
  quality_score: number;
  usage_count: number;
  success_count: number;
  failure_count: number;
  last_used_at: string;
  pinned: boolean;
  archived: boolean;
}

export interface GovernanceOverview {
  total: number;
  active: number;
  archived: number;
  pinned: number;
  avg_quality: number;
  stale_skills: SkillQualityInfo[];
  top_skills: SkillQualityInfo[];
}
