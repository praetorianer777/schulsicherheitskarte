import { useState } from "react";

import type { Accident } from "../api/types";
import { formatNumber, severityLabel, weekdayLabel } from "../lib/format";

type Props = { accidents: Accident[] };

const pageSize = 25;

/**
 * The map as a table. Everything drawn as a dot is here in words, which is what
 * makes the map optional rather than the only way in.
 */
export default function AccidentTable({ accidents }: Props) {
  const [shown, setShown] = useState(pageSize);
  const visible = accidents.slice(0, shown);

  return (
    <section aria-labelledby="unfallliste">
      <h2 id="unfallliste" className="text-lg font-semibold">
        Einzelne Unfälle
      </h2>

      {accidents.length === 0 ? (
        <p className="mt-2 text-ink-muted">Keine Unfälle im gewählten Umkreis und Zeitraum.</p>
      ) : (
        <>
          <div className="mt-2 overflow-x-auto rounded border border-line bg-white">
            <table className="w-full text-left text-sm">
              <caption className="sr-only">
                Unfälle mit Personenschaden im gewählten Umkreis, nach Schwere sortiert
              </caption>
              <thead className="border-b border-line">
                <tr>
                  <th scope="col" className="px-3 py-2">Folgen</th>
                  <th scope="col" className="px-3 py-2">Jahr</th>
                  <th scope="col" className="px-3 py-2">Wochentag</th>
                  <th scope="col" className="px-3 py-2">Uhrzeit</th>
                  <th scope="col" className="px-3 py-2">Beteiligt</th>
                  <th scope="col" className="px-3 py-2">Entfernung</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {visible.map((accident) => (
                  <tr key={accident.id}>
                    <td className="px-3 py-2">{severityLabel[accident.severity]}</td>
                    <td className="px-3 py-2 tabular-nums">{accident.year}</td>
                    <td className="px-3 py-2">{weekdayLabel(accident.weekday)}</td>
                    <td className="px-3 py-2 tabular-nums">{String(accident.hour).padStart(2, "0")} Uhr</td>
                    <td className="px-3 py-2">{involvement(accident)}</td>
                    <td className="px-3 py-2 tabular-nums">{formatNumber(accident.distance)} m</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {shown < accidents.length && (
            <button
              type="button"
              onClick={() => setShown(shown + pageSize)}
              className="mt-2 rounded border border-line bg-white px-3 py-2"
            >
              Weitere {Math.min(pageSize, accidents.length - shown)} von{" "}
              {formatNumber(accidents.length - shown)} anzeigen
            </button>
          )}
        </>
      )}
    </section>
  );
}

function involvement(accident: Accident): string {
  const parts: string[] = [];
  if (accident.pedestrian) parts.push("zu Fuß");
  if (accident.bike) parts.push("Rad");
  if (accident.motorcycle) parts.push("Krad");
  if (accident.car) parts.push("Pkw");
  if (accident.truck) parts.push("Lkw");
  return parts.join(", ");
}
