"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronRight } from "lucide-react";
import { agentTaskSnapshotOptions } from "@multica/core/agents/queries";
import { issueDetailOptions } from "@multica/core/issues/queries";
import { agentListOptions as wsAgentListOptions } from "@multica/core/workspace/queries";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@multica/ui/components/ui/collapsible";
import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from "@multica/ui/components/ui/sidebar";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { AppLink } from "../navigation";
import { useWorkspacePaths } from "@multica/core/paths";
import { useT } from "../i18n";
import type { AgentTask } from "@multica/core/types";

/**
 * Dot that pulses green for running tasks, static orange for queued/dispatched.
 */
function StatusDot({ status }: { status: AgentTask["status"] }) {
  if (status === "running") {
    return (
      <span className="relative ml-auto shrink-0 flex size-2 items-center justify-center">
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75" />
        <span className="relative inline-flex size-1.5 rounded-full bg-green-500" />
      </span>
    );
  }
  return <span className="ml-auto shrink-0 size-1.5 rounded-full bg-orange-400" />;
}

/**
 * Pulsing green dot shown next to the "Active" section label when tasks exist.
 */
function PulsingDot() {
  return (
    <span className="relative mr-1 flex size-2 items-center justify-center">
      <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75" />
      <span className="relative inline-flex size-1.5 rounded-full bg-green-500" />
    </span>
  );
}

/**
 * Sidebar section that shows all active (running / queued / dispatched) agent
 * tasks in the current workspace. Always rendered so users learn this is where
 * agent activity surfaces — empty state shown when no tasks are running.
 *
 * Data freshness is driven by WS task events (task:dispatch, task:running,
 * task:completed, etc.) which already invalidate agentTaskSnapshotOptions via
 * useRealtimeSync — no additional wiring required.
 */
export function ActiveTasksSidebarSection({ wsId }: { wsId: string }) {
  const { t } = useT("layout");
  const p = useWorkspacePaths();
  const qc = useQueryClient();

  const { data: snapshot = [] } = useQuery(agentTaskSnapshotOptions(wsId));
  const { data: agents = [] } = useQuery(wsAgentListOptions(wsId));

  const activeTasks = snapshot.filter(
    (task) => task.status === "running" || task.status === "queued" || task.status === "dispatched",
  );

  return (
    <Collapsible defaultOpen>
      <SidebarGroup>
        <SidebarGroupLabel
          render={<CollapsibleTrigger />}
          className="group/trigger cursor-pointer hover:bg-sidebar-accent/70 hover:text-sidebar-accent-foreground"
        >
          {activeTasks.length > 0 && <PulsingDot />}
          <span>{t(($) => $.sidebar.active_label)}</span>
          <ChevronRight className="!size-3 ml-1 stroke-[2.5] transition-transform duration-200 group-data-[panel-open]/trigger:rotate-90" />
          {activeTasks.length > 0 && (
            <span className="ml-auto text-[10px] text-muted-foreground">{activeTasks.length}</span>
          )}
        </SidebarGroupLabel>
        <CollapsibleContent>
          <SidebarGroupContent>
            {activeTasks.length === 0 ? (
              <p className="px-2 text-xs text-muted-foreground">
                {t(($) => $.sidebar.no_agents_working)}
              </p>
            ) : (
              <SidebarMenu className="gap-0.5">
                {activeTasks.map((task) => {
                  const agent = agents.find((a) => a.id === task.agent_id);
                  const cached = task.issue_id
                    ? qc.getQueryData<{ identifier?: string }>(
                        issueDetailOptions(wsId, task.issue_id).queryKey,
                      )
                    : undefined;
                  const identifier = cached?.identifier ?? (task.issue_id ? task.issue_id.slice(0, 8) : undefined);
                  const canNavigate = !!task.issue_id;

                  return (
                    <SidebarMenuItem key={task.id}>
                      <SidebarMenuButton
                        size="sm"
                        render={canNavigate ? <AppLink href={p.issueDetail(task.issue_id)} /> : <span />}
                        className="text-muted-foreground hover:not-data-active:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground"
                      >
                        <ActorAvatar
                          name={agent?.name ?? ""}
                          initials={(agent?.name ?? "A").charAt(0).toUpperCase()}
                          avatarUrl={agent?.avatar_url ?? null}
                          size={16}
                        />
                        <span
                          className="min-w-0 flex-1 overflow-hidden whitespace-nowrap"
                          style={{
                            maskImage: "linear-gradient(to right, black calc(100% - 12px), transparent)",
                            WebkitMaskImage: "linear-gradient(to right, black calc(100% - 12px), transparent)",
                          }}
                        >
                          {agent?.name ?? "—"}
                        </span>
                        {identifier && (
                          <span className="shrink-0 text-[10px] text-muted-foreground">{identifier}</span>
                        )}
                        <StatusDot status={task.status} />
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}
              </SidebarMenu>
            )}
          </SidebarGroupContent>
        </CollapsibleContent>
      </SidebarGroup>
    </Collapsible>
  );
}
