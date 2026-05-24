"use client";

import { AgentDojoPage } from "@multica/views/dojo/agent-dojo-page";
import { ErrorBoundary } from "@multica/ui/components/common/error-boundary";

export default function Page() {
  return (
    <ErrorBoundary>
      <AgentDojoPage />
    </ErrorBoundary>
  );
}
