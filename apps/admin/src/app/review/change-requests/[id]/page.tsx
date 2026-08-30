import { ChangeRequestDetail } from "@/components/review-workspace";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ChangeRequestDetail id={id} />;
}
