import type { AccidentCounts } from "../api/types";
import { formatNumber } from "../lib/format";

type Props = { summary: AccidentCounts; radius: number; from: number; to: number };

export default function KeyFigures({ summary, radius, from, to }: Props) {
  return (
    <section aria-labelledby="kennzahlen">
      <h2 id="kennzahlen" className="text-lg font-semibold">
        Unfälle im Umkreis von {radius} m, {from}–{to}
      </h2>

      <dl className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
        <Figure label="Unfälle insgesamt" value={summary.total} />
        <Figure label="mit Getöteten" value={summary.fatal} emphasise={summary.fatal > 0} />
        <Figure label="mit Schwerverletzten" value={summary.serious} />
        <Figure label="zu Fuß beteiligt" value={summary.pedestrian} />
        <Figure label="mit dem Rad beteiligt" value={summary.bike} />
        <Figure label="mit Leichtverletzten" value={summary.slight} />
      </dl>

      {summary.total === 0 && (
        <p className="mt-3 text-ink-muted">
          In diesem Umkreis und Zeitraum ist kein Unfall mit Personenschaden erfasst. Das ist
          kein Beleg dafür, dass der Weg sicher ist — Beinahe-Unfälle tauchen in keiner
          Statistik auf.
        </p>
      )}
    </section>
  );
}

function Figure({ label, value, emphasise }: { label: string; value: number; emphasise?: boolean }) {
  return (
    <div className="rounded border border-line bg-white px-3 py-2">
      <dt className="text-sm text-ink-muted">{label}</dt>
      <dd className={`text-2xl font-semibold ${emphasise ? "text-critical" : ""}`}>
        {formatNumber(value)}
      </dd>
    </div>
  );
}
