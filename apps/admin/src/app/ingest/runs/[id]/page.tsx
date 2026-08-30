import { SourceRunDetail, type EvidenceKind } from "@/components/source-run-detail";

export default async function Page({ params, searchParams }: { params: Promise<{ id: string }>; searchParams: Promise<{ evidence?: string }> }) {
  const { id } = await params;
  const { evidence } = await searchParams;
  const initialKind: EvidenceKind = evidence === "conflicts" || evidence === "duplicates" ? evidence : "records";
  return <SourceRunDetail id={id} initialKind={initialKind} />;
}
