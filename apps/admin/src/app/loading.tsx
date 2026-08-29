export default function Loading() {
  return (
    <section className="admin-splash" aria-label="Loading admin workspace" aria-live="polite">
      <div className="admin-splash__scene" aria-hidden>
        <span className="admin-splash__map">
          <i className="admin-splash__road admin-splash__road--one" />
          <i className="admin-splash__road admin-splash__road--two" />
          <i className="admin-splash__place admin-splash__place--one" />
          <i className="admin-splash__place admin-splash__place--two" />
          <i className="admin-splash__place admin-splash__place--three" />
        </span>
        <span className="admin-splash__marker">GG</span>
      </div>
      <p>Finding your place…</p>
      <span>Loading the steward workspace</span>
    </section>
  );
}
