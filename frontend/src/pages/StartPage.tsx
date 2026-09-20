import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";

import { api } from "../api/client";
import SearchBox from "../components/SearchBox";
import { institutionName, kindLabel } from "../lib/format";

export default function StartPage() {
  const [query, setQuery] = useState("");

  const search = useQuery({
    queryKey: ["institutions", query],
    queryFn: () => api.searchInstitutions(query),
    enabled: query.length >= 2,
  });

  const results = search.data?.institutions ?? [];
  const nothingImported = search.data?.nothingImported ?? false;

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      <h1 className="text-2xl font-semibold">Wie sicher ist der Weg zu dieser Schule?</h1>
      <p className="mt-3 text-ink-muted">
        Diese Karte führt die Unfälle mit Personenschaden aus dem Unfallatlas mit den Schulen,
        Kitas, Querungshilfen und Tempolimits aus OpenStreetMap zusammen — damit
        Elternvertretungen mit Zahlen argumentieren können statt mit Anekdoten.
      </p>

      <div className="mt-6">
        <SearchBox onSearch={setQuery} busy={search.isFetching} />
      </div>

      {/* Results arrive after the request, so a screen reader has to be told
          that something appeared below the field it just left. */}
      <div aria-live="polite" className="mt-6">
        {query.length >= 2 && search.isFetching && <p>Wird gesucht …</p>}

        {search.isError && (
          <p className="text-critical">
            Die Suche ist fehlgeschlagen: {(search.error as Error).message}
          </p>
        )}

        {search.isSuccess && results.length === 0 && nothingImported && (
          <div className="rounded border border-line bg-white p-4">
            <h2 className="text-lg font-semibold">Es sind noch keine Daten importiert</h2>
            <p className="mt-2">
              Die Datenbank ist leer — deshalb findet die Suche nichts, egal wonach gesucht wird.
              Wer diese Installation betreibt, muss den Import einmal ausführen; er ist in
              DEPLOY.md in Abschnitt 3 beschrieben.
            </p>
          </div>
        )}

        {search.isSuccess && results.length === 0 && !nothingImported && (
          <p>
            Zu „{query}“ wurde nichts gefunden. Möglicherweise liegt die Einrichtung außerhalb
            des importierten Gebiets, oder sie ist in OpenStreetMap ohne Namen erfasst.
          </p>
        )}

        {results.length > 0 && (
          <>
            <h2 className="text-lg font-semibold">
              {results.length} {results.length === 1 ? "Treffer" : "Treffer"}
            </h2>
            <ul className="mt-2 divide-y divide-line rounded border border-line bg-white">
              {results.map((institution) => (
                <li key={institution.id}>
                  <Link
                    to={`/einrichtung/${institution.id}`}
                    className="flex items-baseline justify-between gap-4 px-4 py-3 no-underline hover:bg-surface"
                  >
                    <span className="font-medium">{institutionName(institution)}</span>
                    <span className="text-sm text-ink-muted">{kindLabel[institution.kind]}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </div>
  );
}
