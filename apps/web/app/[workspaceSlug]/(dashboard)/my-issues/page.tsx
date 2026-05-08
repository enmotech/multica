import { redirect } from "next/navigation";

interface Props {
  params: Promise<{ workspaceSlug: string }>;
}

export default async function MyIssuesRedirect({ params }: Props) {
  const { workspaceSlug } = await params;
  redirect(`/${workspaceSlug}/my-tasks`);
}
