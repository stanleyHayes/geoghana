import { Card, Skeleton } from "@ghanageo/ui";

export default function Loading() {
  return (
    <section className="admin-page-skeleton" aria-label="Loading admin workspace" aria-live="polite">
      <div className="admin-page-skeleton__title">
        <Skeleton /><Skeleton />
      </div>
      <div className="admin-page-skeleton__stats">
        {[0, 1, 2, 3].map((item) => <Card key={item}><Skeleton /><Skeleton /><Skeleton /></Card>)}
      </div>
      <Card className="admin-page-skeleton__table">
        <Skeleton />
        {[0, 1, 2, 3, 4, 5].map((row) => <div key={row}><Skeleton /><Skeleton /><Skeleton /></div>)}
      </Card>
    </section>
  );
}
