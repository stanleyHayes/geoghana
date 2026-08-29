import { ArrowLeft, Compass, MapPin, Search } from "lucide-react";
import Link from "next/link";

export default function NotFound() {
  return (
    <section className="admin-not-found" aria-labelledby="not-found-title">
      <div className="admin-not-found__art" aria-hidden>
        <span className="admin-not-found__orbit" />
        <span className="admin-not-found__pin">
          <MapPin size={34} />
        </span>
        <span className="admin-not-found__dot admin-not-found__dot--one" />
        <span className="admin-not-found__dot admin-not-found__dot--two" />
        <span className="admin-not-found__code">404</span>
      </div>

      <p className="admin-not-found__eyebrow">Outside the known boundary</p>
      <h1 id="not-found-title">This place is not on the admin map.</h1>
      <p className="admin-not-found__lede">
        The address may be mistyped, moved, or not part of the steward
        workspace. No data was changed.
      </p>

      <div className="admin-not-found__actions">
        <Link className="gg-button gg-button--primary gg-button--md" href="/">
          <ArrowLeft size={16} aria-hidden /> Back to overview
        </Link>
        <Link
          className="gg-button gg-button--secondary gg-button--md"
          href="/explorer"
        >
          <Compass size={16} aria-hidden /> Open explorer
        </Link>
      </div>

      <p className="admin-not-found__hint">
        <Search size={14} aria-hidden /> You can also press{" "}
        <kbd className="gg-kbd">⌘K</kbd> to find a screen or place.
      </p>
    </section>
  );
}
