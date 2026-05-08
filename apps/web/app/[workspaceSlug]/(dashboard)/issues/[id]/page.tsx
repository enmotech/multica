import { redirect } from "next/navigation";

interface Props {
  params: Promise<{ workspaceSlug: string; id: string }>;
}

export default async function IssueDetailRedirect({ params }: Props) {
  const { workspaceSlug, id } = await params;
  redirect(`/${workspaceSlug}/tasks/${id}`);
}
