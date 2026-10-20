import { Link } from "react-router-dom";

/** Fallback route for unknown paths. */
export function NotFoundPage() {
  return (
    <section aria-labelledby="not-found-heading">
      <h1 id="not-found-heading" className="text-2xl font-semibold">
        Page not found
      </h1>
      <p className="mt-1 text-sm text-gray-600">
        The requested page does not exist in this shell.
      </p>
      <Link
        to="/"
        className="mt-4 inline-block rounded text-sm text-blue-700 underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
      >
        Return to Overview
      </Link>
    </section>
  );
}
