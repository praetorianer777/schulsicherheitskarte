import { useQuery } from "@tanstack/react-query";
import { useCallback, useState } from "react";
import { Link, useNavigate } from "react-router-dom";

import { api } from "../api/client";
import type { Institution } from "../api/types";
import OverviewMap, { type BBox } from "../components/OverviewMap";
import SearchBox from "../components/SearchBox";
import { institutionName, kindLabel } from "../lib/format";

export default function StartPage() {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [view, setView] = useState<BBox | null>(null);
  const [focus, setFocus] = useState<Institution[] | null>(null);

  const extent = useQuery({ queryKey: ["extent"], queryFn: api.extent });

  const search = useQuery({
    queryKey: ["institutions", query],
    queryFn: () => api.searchInstitutions(query),
    enabled: query.length >= 2,
  });

  // What is inside the map right now. Keyed by the view, so panning back to a
  // place already seen costs nothing.
  const inView = useQuery({
    queryKey: ["institutions-in-view", view?.map((v) => v.toFixed(4)).join(",")],
    queryFn: () => api.institutionsInView(view!),
    enabled: view !== null,
    placeholderData: (previous) => previous,
  });

  const results = search.data?.institutions ?? [];
  const nothingImported = search.data?.nothingImported ?? false;
  const empty = extent.isSuccess && extent.data.institutions === 0;

  const onSearch = (next: string) => {
    setQuery(next);
    setFocus(null);
  };
  // The hits frame the map once they are in; a stale focus from the previous
  // search must not move it first.
  if (search.isSuccess && results.length > 0 && focus === null && query.length >= 2) {
    setFocus(results);
  }

  const onViewChange = useCallback((bbox: BBox) => setView(bbox), []);
  const onSelect = useCallback((id: number) => navigate(`/einrichtung/${id}`), [navigate]);

  return (
    <div className="mx-auto max-w-6xl px-4 py-6">
      <h1 className="text-2xl font-semibold">Wie sicher ist der Weg zu dieser Schule?</h1>
      <p className="mt-3 max-w-3xl text-ink-muted">
        Diese Karte führt die Unfälle mit Personenschaden aus dem Unfallatlas mit den Schulen,
        Kitas, Querungshilfen und Tempolimits aus OpenStreetMap zusammen — damit
        Elternvertretungen mit Zahlen argumentieren können statt mit Anekdoten.
      </p>

      {empty ? (
        <div className="mt-6 rounded border border-line bg-white p-4">
          <h2 className="text-lg font-semibold">Es sind noch keine Daten importiert</h2>
          <p className="mt-2">
            Die Datenbank ist leer — deshalb gibt es weder Karte noch Suche. Wer diese
            Installation betreibt, muss den Import einmal ausführen; er ist in DEPLOY.md in
            Abschnitt 3 beschrieben.
          </p>
        </div>
      ) : (
        <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(20rem,1fr)_2fr]">
          {/* The search comes first in the document: on a phone it sits above the
              map, and the result list is reachable without scrolling past it. */}
          <div>
            <SearchBox onSearch={onSearch} busy={search.isFetching} />

            <div aria-live="polite" className="mt-4">
              {query.length >= 2 && search.isFetching && <p>Wird gesucht …</p>}

              {search.isError && (
                <p className="text-critical">
                  Die Suche ist fehlgeschlagen: {(search.error as Error).message}
                </p>
              )}

              {search.isSuccess && results.length === 0 && nothingImported && (
                <p>Es sind noch keine Daten importiert, deshalb findet die Suche nichts.</p>
              )}

              {search.isSuccess && results.length === 0 && !nothingImported && (
                <p>
                  Zu „{query}“ wurde nichts gefunden. Möglicherweise liegt die Einrichtung
                  außerhalb des importierten Gebiets, oder sie ist in OpenStreetMap ohne Namen
                  erfasst.
                </p>
              )}

              {results.length > 0 && (
                <>
                  <h2 className="text-lg font-semibold">{results.length} Treffer</h2>
                  <ul className="mt-2 divide-y divide-line rounded border border-line bg-white">
                    {results.map((institution) => (
                      <li
                        key={institution.id}
                        className="flex items-baseline justify-between gap-3 px-4 py-3"
                      >
                        <Link to={`/einrichtung/${institution.id}`} className="no-underline">
                          <span className="block font-medium">{institutionName(institution)}</span>
                          <span className="block text-sm text-ink-muted">
                            {kindLabel[institution.kind]}
                            {institution.town ? ` · ${institution.town}` : ""}
                          </span>
                        </Link>
                        <button
                          type="button"
                          onClick={() => setFocus([institution])}
                          className="shrink-0 text-sm text-accent"
                        >
                          Auf der Karte zeigen
                        </button>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </div>

            {extent.isSuccess && extent.data.bbox && (
              <p className="mt-4 text-sm text-ink-muted">
                {extent.data.institutions} Schulen und Kitas im importierten Gebiet. Ein Punkt auf
                der Karte öffnet die Einrichtung; die Liste hier führt zu denselben Seiten.
              </p>
            )}
          </div>

          {extent.isSuccess && extent.data.bbox && (
            <OverviewMap
              extent={extent.data.bbox}
              institutions={inView.data?.institutions ?? []}
              focus={focus}
              onViewChange={onViewChange}
              onSelect={onSelect}
            />
          )}
        </div>
      )}
    </div>
  );
}
