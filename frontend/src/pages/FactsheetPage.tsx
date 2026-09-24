import { useQuery } from "@tanstack/react-query";
import { Link, useParams, useSearchParams } from "react-router-dom";

import { api } from "../api/client";
import type { Factsheet } from "../api/types";
import FactsheetMap from "../components/FactsheetMap";
import { formatNumber, infrastructureLabel, institutionName, kindLabel } from "../lib/format";

/**
 * The page somebody prints and takes to a road safety inspection.
 *
 * The filters are read from the URL rather than from component state, so the
 * sheet can be linked, re-opened and printed again with exactly the numbers it
 * was produced with.
 */
export default function FactsheetPage() {
  const { id } = useParams();
  const institutionId = Number(id);
  const [params] = useSearchParams();

  const filter = {
    radius: Number(params.get("radius") ?? 500),
    from: Number(params.get("from") ?? 2016),
    to: Number(params.get("to") ?? new Date().getFullYear()),
    modes: params.get("modes") ?? undefined,
  };

  const sheet = useQuery({
    queryKey: ["factsheet", institutionId, filter],
    queryFn: () => api.factsheet(institutionId, filter),
    enabled: Number.isFinite(institutionId),
  });

  if (sheet.isError) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-10">
        <h1 className="text-2xl font-semibold">Faktenblatt nicht verfügbar</h1>
        <p className="mt-2">{(sheet.error as Error).message}</p>
      </div>
    );
  }

  if (!sheet.data) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-10" aria-live="polite">
        <p>Faktenblatt wird erstellt …</p>
      </div>
    );
  }

  return <Sheet sheet={sheet.data} />;
}

function Sheet({ sheet }: { sheet: Factsheet }) {
  const { institution, summary, method } = sheet;
  const today = new Date().toLocaleDateString("de-DE", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });

  return (
    <div className="mx-auto max-w-[210mm] px-4 py-6 print:max-w-none print:px-0 print:py-0">
      <div className="no-print mb-4 flex flex-wrap items-center gap-3">
        <Link to={`/einrichtung/${institution.id}`}>← Zurück zur Karte</Link>
        <button
          type="button"
          onClick={() => window.print()}
          className="rounded bg-accent px-4 py-2 font-medium text-white"
        >
          Drucken
        </button>
        <span className="text-sm text-ink-muted">
          Umkreis und Zeitraum stehen in der Adresse dieser Seite — derselbe Link ergibt
          dieselben Zahlen.
        </span>
      </div>

      <article className="factsheet space-y-4 bg-white p-6 text-[10.5pt] leading-snug print:p-0">
        <header>
          <p className="text-sm text-ink-muted">An die Straßenverkehrsbehörde</p>
          <h1 className="mt-1 text-xl font-semibold">
            Schulwegsicherheit: {institutionName(institution)}
          </h1>
          <p className="text-sm text-ink-muted">
            {kindLabel[institution.kind]}
            {institution.schoolType ? ` · ${institution.schoolType}` : ""}
            {institution.town ? ` · ${institution.town}` : ""} · Umkreis{" "}
            {sheet.radius} m · Berichtsjahre {sheet.years.from}–{sheet.years.to} · Stand {today}
          </p>
        </header>

        <FactsheetMap institution={institution} hotspots={sheet.hotspots} radius={sheet.radius} />

        <section>
          <h2 className="text-base font-semibold">Unfälle mit Personenschaden im Umkreis</h2>
          <table className="mt-1 w-full border-collapse text-left">
            <tbody>
              <Row label="Unfälle insgesamt" value={summary.total} />
              <Row label="davon mit Getöteten" value={summary.fatal} />
              <Row label="davon mit Schwerverletzten" value={summary.serious} />
              <Row label="davon mit Leichtverletzten" value={summary.slight} />
              <Row label="mit Beteiligung zu Fuß" value={summary.pedestrian} />
              <Row label="mit Beteiligung mit dem Rad" value={summary.bike} />
            </tbody>
          </table>
        </section>

        <section className="break-inside-avoid">
          <h2 className="text-base font-semibold">Unfallschwerpunkte</h2>
          {sheet.hotspots.length === 0 ? (
            <p className="mt-1">
              Im gewählten Umkreis gibt es keine Stelle mit mindestens{" "}
              {method.clusterMinAccidents} Unfällen innerhalb von {method.clusterRadiusMetres} m.
            </p>
          ) : (
            <table className="mt-1 w-full border-collapse text-left">
              <thead>
                <tr className="border-b border-line">
                  <th scope="col" className="py-1 pr-2">Rang</th>
                  <th scope="col" className="py-1 pr-2">Unfälle</th>
                  <th scope="col" className="py-1 pr-2">davon Fuß / Rad</th>
                  <th scope="col" className="py-1 pr-2">Zeitraum</th>
                  <th scope="col" className="py-1 pr-2">Entfernung</th>
                  <th scope="col" className="py-1">Gefahrenindex</th>
                </tr>
              </thead>
              <tbody>
                {sheet.hotspots.map((hotspot, index) => (
                  <tr key={hotspot.id} className="border-b border-line">
                    <td className="py-1 pr-2 tabular-nums">{index + 1}</td>
                    <td className="py-1 pr-2 tabular-nums">{formatNumber(hotspot.accidentCount)}</td>
                    <td className="py-1 pr-2 tabular-nums">
                      {formatNumber(hotspot.breakdown.pedestrian ?? 0)} /{" "}
                      {formatNumber(hotspot.breakdown.bike ?? 0)}
                    </td>
                    <td className="py-1 pr-2 tabular-nums">
                      {hotspot.firstYear}–{hotspot.lastYear}
                    </td>
                    <td className="py-1 pr-2 tabular-nums">{formatNumber(hotspot.distance)} m</td>
                    <td className="py-1 tabular-nums">{hotspot.score.toFixed(1)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        <section className="break-inside-avoid">
          <h2 className="text-base font-semibold">Vorhandene Infrastruktur im Umkreis</h2>
          <p className="mt-1">
            {Object.entries(sheet.infrastructureCounts).length === 0
              ? "In OpenStreetMap ist im Umkreis nichts davon erfasst."
              : Object.entries(sheet.infrastructureCounts)
                  .map(
                    ([kind, count]) =>
                      `${formatNumber(count)} × ${infrastructureLabel[kind as keyof typeof infrastructureLabel] ?? kind}`,
                  )
                  .join(", ")}
          </p>
          <p className="mt-1 text-ink-muted">
            Stand von OpenStreetMap. Was dort nicht erfasst ist, fehlt hier — das Gegenteil
            gilt nicht.
          </p>
        </section>

        <section className="break-inside-avoid">
          <h2 className="text-base font-semibold">Wie der Gefahrenindex berechnet wird</h2>
          <p className="mt-1">
            Ein Schwerpunkt ist ein Kreis von {method.clusterRadiusMetres} m Radius mit
            mindestens {method.clusterMinAccidents} Unfällen. Sein Index ist die Summe über
            diese Unfälle:
          </p>
          <p className="mt-1">
            Getötete × {method.weightFatal}, Schwerverletzte × {method.weightSerious},
            Leichtverletzte × {method.weightSlight}; zusätzlich × {method.weightVulnerable}, wenn
            jemand zu Fuß oder mit dem Rad beteiligt war; halbiert alle{" "}
            {method.halfLifeYears} Jahre, gerechnet ab dem Berichtsjahr {method.referenceYear}.
          </p>
        </section>

        <section className="break-inside-avoid">
          <h2 className="text-base font-semibold">Was diese Zahlen nicht hergeben</h2>
          <ul className="mt-1 list-disc space-y-0.5 pl-5">
            <li>
              Erfasst sind nur Unfälle mit Personenschaden. Beinahe-Unfälle, Sachschäden und
              alltägliche Bedrängung tauchen nicht auf.
            </li>
            <li>
              Die Koordinaten sind auf das Straßennetz gerastert und anonymisiert. Ein Punkt
              bezeichnet einen Straßenabschnitt, keine Stelle auf dem Asphalt.
            </li>
            <li>
              Es gibt keine Angaben zur Verkehrsmenge. Das sind absolute Häufigkeiten, keine
              Risikoraten: Eine ruhige Straße mit einem Unfall ist nicht automatisch sicherer
              als eine stark befahrene mit dreien.
            </li>
            <li>
              Dass an einer Stelle nichts passiert ist, belegt nicht, dass sie sicher ist.
            </li>
          </ul>
        </section>

        <footer className="break-inside-avoid border-t border-line pt-2 text-sm text-ink-muted">
          <p>Quellen:</p>
          <ul className="list-disc pl-5">
            {sheet.sources.map((source) => (
              <li key={source.url}>
                {source.name} ({source.licence}), {source.url}
              </li>
            ))}
            <li>Kartengrundlage: © basemap.de / BKG</li>
          </ul>
        </footer>
      </article>
    </div>
  );
}

function Row({ label, value }: { label: string; value: number }) {
  return (
    <tr className="border-b border-line">
      <th scope="row" className="py-1 pr-4 text-left font-normal">
        {label}
      </th>
      <td className="py-1 text-right tabular-nums font-semibold">{formatNumber(value)}</td>
    </tr>
  );
}
