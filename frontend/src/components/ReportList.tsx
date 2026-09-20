import type { Report } from "../api/types";
import { formatNumber, reportCategoryLabel } from "../lib/format";

type Props = {
  reports: Report[];
  onConfirm: (report: Report) => void;
  confirmed: Set<string>;
};

/**
 * The reported places, in words — the same content the markers carry, and the
 * only way to confirm one without a mouse.
 */
export default function ReportList({ reports, onConfirm, confirmed }: Props) {
  return (
    <section aria-labelledby="meldungen">
      <h2 id="meldungen" className="text-lg font-semibold">
        Gemeldete Gefahrenstellen
      </h2>
      <p className="mt-1 text-ink-muted">
        Von Eltern gemeldet und vor der Veröffentlichung gesichtet. Sie ergänzen die
        Unfallstatistik um das, was dort nie ankommt: Stellen, an denen es beinahe passiert.
      </p>

      {reports.length === 0 ? (
        <p className="mt-2 text-ink-muted">
          Für diesen Umkreis ist noch nichts gemeldet — oder noch nichts freigegeben.
        </p>
      ) : (
        <ul className="mt-2 divide-y divide-line rounded border border-line bg-white">
          {reports.map((report) => (
            <li key={report.id} className="px-4 py-3">
              <p className="font-medium">{reportCategoryLabel[report.category] ?? report.category}</p>
              {report.description && <p className="mt-1">{report.description}</p>}
              <p className="mt-1 text-sm text-ink-muted">
                {formatNumber(report.confirmations)}{" "}
                {report.confirmations === 1 ? "Person bestätigt" : "Personen bestätigen"} das auch
              </p>
              <button
                type="button"
                onClick={() => onConfirm(report)}
                disabled={confirmed.has(report.id)}
                className="mt-2 rounded border border-line px-3 py-1 text-sm disabled:opacity-60"
              >
                {confirmed.has(report.id) ? "Danke, notiert" : "Betrifft mich auch"}
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
