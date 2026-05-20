import { LoginPage } from "@mantica/views/auth";
import { ManticaIcon } from "@mantica/ui/components/common/mantica-icon";

export function DesktopLoginPage() {
  return (
    <div className="flex h-screen flex-col">
      {/* Traffic light inset */}
      <div
        className="h-[38px] shrink-0"
        style={{ WebkitAppRegion: "drag" } as React.CSSProperties}
      />
      <LoginPage
        logo={<ManticaIcon bordered size="lg" />}
        onSuccess={() => {
          // Auth store update triggers AppContent re-render → shows DesktopShell
        }}
      />
    </div>
  );
}
