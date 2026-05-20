"use client";

import { DashboardLayout } from "@mantica/views/layout";
import { ManticaIcon } from "@mantica/ui/components/common/mantica-icon";
import { SearchCommand, SearchTrigger } from "@mantica/views/search";
import { ChatFab, ChatWindow } from "@mantica/views/chat";

export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <DashboardLayout
      loadingIndicator={<ManticaIcon className="size-6" />}
      searchSlot={<SearchTrigger />}
      extra={<><SearchCommand /><ChatWindow /><ChatFab /></>}
    >
      {children}
    </DashboardLayout>
  );
}
