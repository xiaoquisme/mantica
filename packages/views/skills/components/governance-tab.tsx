"use client";

import { useEffect, useState } from "react";
import {
  Pin, PinOff, Archive, Trash2,
  AlertTriangle, CheckCircle, RefreshCw,
} from "lucide-react";
import { api } from "@mantica/core/api";
import type { GovernanceOverview, SkillQualityInfo } from "@mantica/core/types";
import { Button } from "@mantica/ui/components/ui/button";
import { toast } from "sonner";

function QualityBadge({ score }: { score: number }) {
  const color =
    score >= 70 ? "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300" :
    score >= 40 ? "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300" :
    "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300";

  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-mono ${color}`}>
      {score.toFixed(0)}
    </span>
  );
}

function SkillCard({
  skill,
  onPin,
  onUnpin,
  onArchive,
}: {
  skill: SkillQualityInfo;
  onPin: () => void;
  onUnpin: () => void;
  onArchive: () => void;
}) {
  const winRate = skill.usage_count > 0
    ? ((skill.success_count / skill.usage_count) * 100).toFixed(0)
    : "—";

  return (
    <div className="flex items-center gap-3 rounded-lg border p-3">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium truncate">{skill.name}</span>
          <QualityBadge score={skill.quality_score} />
          {skill.pinned && (
            <Pin className="h-3 w-3 text-primary" />
          )}
        </div>
        <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
          <span>{skill.usage_count} uses</span>
          <span>{winRate} win</span>
          {skill.last_used_at && (
            <span>Last: {new Date(skill.last_used_at).toLocaleDateString()}</span>
          )}
        </div>
      </div>
      <div className="flex items-center gap-1">
        {skill.pinned ? (
          <Button variant="ghost" size="icon-xs" onClick={onUnpin} title="Unpin">
            <PinOff className="h-3.5 w-3.5" />
          </Button>
        ) : (
          <Button variant="ghost" size="icon-xs" onClick={onPin} title="Pin">
            <Pin className="h-3.5 w-3.5" />
          </Button>
        )}
        {!skill.pinned && (
          <Button variant="ghost" size="icon-xs" onClick={onArchive} title="Archive">
            <Archive className="h-3.5 w-3.5 text-muted-foreground" />
          </Button>
        )}
      </div>
    </div>
  );
}

export function GovernanceTab() {
  const [data, setData] = useState<GovernanceOverview | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchData = () => {
    setLoading(true);
    api.getSkillGovernance().then(setData).catch(() => {}).finally(() => setLoading(false));
  };

  useEffect(() => { fetchData(); }, []);

  const handlePin = async (id: string) => {
    await api.pinSkill(id);
    toast.success("Skill pinned");
    fetchData();
  };

  const handleUnpin = async (id: string) => {
    await api.unpinSkill(id);
    toast.success("Skill unpinned");
    fetchData();
  };

  const handleArchive = async (id: string) => {
    await api.archiveSkill(id);
    toast.success("Skill archived");
    fetchData();
  };

  const handleAutoArchive = async () => {
    const result = await api.autoArchiveSkills();
    toast.success(`Archived ${result.archived} stale skills`);
    fetchData();
  };

  if (loading) {
    return <div className="p-4 text-sm text-muted-foreground">Loading governance data...</div>;
  }

  if (!data) {
    return <div className="p-4 text-sm text-muted-foreground">Failed to load governance data.</div>;
  }

  return (
    <div className="space-y-6 p-4">
      {/* Stats */}
      <div className="grid grid-cols-5 gap-3">
        <div className="text-center">
          <div className="text-2xl font-semibold">{data.total}</div>
          <div className="text-[10px] text-muted-foreground uppercase">Total</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-semibold text-green-600">{data.active}</div>
          <div className="text-[10px] text-muted-foreground uppercase">Active</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-semibold text-gray-400">{data.archived}</div>
          <div className="text-[10px] text-muted-foreground uppercase">Archived</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-semibold text-primary">{data.pinned}</div>
          <div className="text-[10px] text-muted-foreground uppercase">Pinned</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-semibold">{data.avg_quality.toFixed(0)}</div>
          <div className="text-[10px] text-muted-foreground uppercase">Avg Quality</div>
        </div>
      </div>

      {/* Stale Skills */}
      {data.stale_skills.length > 0 && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-medium flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 text-yellow-500" />
              Needs Attention ({data.stale_skills.length})
            </h4>
            <Button variant="outline" size="xs" onClick={handleAutoArchive}>
              <Trash2 className="h-3 w-3 mr-1" />
              Auto-Archive All
            </Button>
          </div>
          <div className="space-y-2">
            {data.stale_skills.map((skill) => (
              <SkillCard
                key={skill.id}
                skill={skill}
                onPin={() => handlePin(skill.id)}
                onUnpin={() => handleUnpin(skill.id)}
                onArchive={() => handleArchive(skill.id)}
              />
            ))}
          </div>
        </div>
      )}

      {/* Top Skills */}
      {data.top_skills.length > 0 && (
        <div className="space-y-3">
          <h4 className="text-sm font-medium flex items-center gap-2">
            <CheckCircle className="h-4 w-4 text-green-500" />
            Top Skills ({data.top_skills.length})
          </h4>
          <div className="space-y-2">
            {data.top_skills.map((skill) => (
              <SkillCard
                key={skill.id}
                skill={skill}
                onPin={() => handlePin(skill.id)}
                onUnpin={() => handleUnpin(skill.id)}
                onArchive={() => handleArchive(skill.id)}
              />
            ))}
          </div>
        </div>
      )}

      {/* Refresh */}
      <div className="flex justify-center">
        <Button variant="ghost" size="sm" onClick={fetchData}>
          <RefreshCw className="h-4 w-4 mr-1" />
          Refresh
        </Button>
      </div>
    </div>
  );
}
