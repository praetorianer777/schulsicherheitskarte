import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useParams } from "react-router-dom";

import { api } from "../api/client";
import type { Hotspot, Report } from "../api/types";
import AccidentTable from "../components/AccidentTable";
import Filters, { type FilterState } from "../components/Filters";
import HotspotList from "../components/HotspotList";
import ReportForm, { type ReportDraft } from "../components/ReportForm";
import ReportList from "../components/ReportList";
import KeyFigures from "../components/KeyFigures";
import Legend from "../components/Legend";
import MapView, { type LayerVisibility } from "../components/MapView";
import { institutionName, kindLabel } from "../lib/format";

/** The reporting years the Unfallatlas has published so far. */
const firstYear = 2016;
const lastYear = new Date().getFullYear();

export default function InstitutionPage() {
  const { id } = useParams();
  const institutionId = Number(id);

  const [filters, setFilters] = useState<FilterState>({
    radius: 500,
    from: firstYear,
    to: lastYear,
    onlyVulnerable: false,
  });
  const [layers, setLayers] = useState<LayerVisibility>({
    accidents: true,
    hotspots: true,
    infrastructure: false,
    reports: true,
  });
  const [focus, setFocus] = useState<{ lon: number; lat: number } | null>(null);
  const [draft, setDraft] = useState<{ lon: number; lat: number } | null>(null);
  const [confirmed, setConfirmed] = useState<Set<string>>(new Set());
  const client = useQueryClient();

  const institution = useQuery({
    queryKey: ["institution", institutionId],
    queryFn: () => api.institution(institutionId),
    enabled: Number.isFinite(institutionId),
  });

  const accidents = useQuery({
    queryKey: ["accidents", institutionId, filters],
    queryFn: () =>
      api.accidents(institutionId, {
        radius: filters.radius,
        from: filters.from,
        to: filters.to,
        modes: filters.onlyVulnerable ? "foot,bike" : undefined,
      }),
    enabled: Number.isFinite(institutionId),
  });

  const hotspots = useQuery({
    queryKey: ["hotspots", institutionId, filters.radius],
    queryFn: () => api.hotspots(institutionId, filters.radius),
    enabled: Number.isFinite(institutionId),
  });

  const reports = useQuery({
    queryKey: ["reports", institutionId, filters.radius],
    queryFn: () => api.reportsNear(institutionId, filters.radius),
    enabled: Number.isFinite(institutionId),
  });

  const submitReport = useMutation({
    mutationFn: (report: ReportDraft) => api.createReport(report),
    onSuccess: () => client.invalidateQueries({ queryKey: ["reports", institutionId] }),
  });

  const confirmReport = useMutation({
    mutationFn: (report: Report) => api.confirmReport(report.id),
    onSuccess: (_result, report) => {
      setConfirmed((previous) => new Set(previous).add(report.id));
      client.invalidateQueries({ queryKey: ["reports", institutionId] });
    },
  });

  const infrastructure = useQuery({
    queryKey: ["infrastructure", institutionId, filters.radius],
    queryFn: () => api.infrastructure(institutionId, filters.radius),
    enabled: Number.isFinite(institutionId) && layers.infrastructure,
  });

  if (institution.isError) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-12">
        <h1 className="text-2xl font-semibold">Einrichtung nicht gefunden</h1>
        <p className="mt-2 text-ink-muted">{(institution.error as Error).message}</p>
        <p className="mt-4">
          <Link to="/">Zurück zur Suche</Link>
        </p>
      </div>
    );
  }

  if (!institution.data) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-12" aria-live="polite">
        <p>Einrichtung wird geladen …</p>
      </div>
    );
  }

  const current = institution.data.institution;
  const accidentList = accidents.data?.accidents ?? [];
  const summary = accidents.data?.summary ?? {
    total: 0,
    fatal: 0,
    serious: 0,
    slight: 0,
    pedestrian: 0,
    bike: 0,
  };

  const selectHotspot = (hotspot: Hotspot) => setFocus({ lon: hotspot.lon, lat: hotspot.lat });

  return (
    <div className="mx-auto max-w-6xl px-4 py-6">
      <p className="text-sm">
        <Link to="/">← Zurück zur Suche</Link>
      </p>

      <h1 className="mt-2 text-2xl font-semibold">{institutionName(current)}</h1>
      <p className="text-ink-muted">
        {kindLabel[current.kind]}
        {current.schoolType ? ` · ${current.schoolType}` : ""}
      </p>

      <div className="mt-6 grid gap-6 lg:grid-cols-[2fr_1fr]">
        <div className="space-y-4">
          <MapView
            institution={current}
            accidents={layers.accidents ? accidentList : []}
            hotspots={layers.hotspots ? (hotspots.data?.hotspots ?? []) : []}
            infrastructure={layers.infrastructure ? (infrastructure.data?.infrastructure ?? []) : []}
            reports={layers.reports ? (reports.data?.reports ?? []) : []}
            radius={filters.radius}
            layers={layers}
            focus={focus}
            draft={draft}
            onPick={draft ? setDraft : undefined}
          />
          <Legend />
        </div>

        <div className="space-y-4">
          <Filters
            value={filters}
            onChange={setFilters}
            layers={layers}
            onLayersChange={setLayers}
            years={{ first: firstYear, last: lastYear }}
          />

          {draft ? (
            <ReportForm
              institution={current}
              point={draft}
              onPointChange={setDraft}
              onSubmit={(report) => submitReport.mutate(report)}
              onCancel={() => {
                setDraft(null);
                submitReport.reset();
              }}
              busy={submitReport.isPending}
              submitted={submitReport.isSuccess}
              error={submitReport.isError ? (submitReport.error as Error).message : null}
            />
          ) : (
            <button
              type="button"
              onClick={() => setDraft({ lon: current.lon, lat: current.lat })}
              className="w-full rounded border border-line bg-white px-4 py-3 text-left"
            >
              <span className="font-medium">Gefahrenstelle melden</span>
              <span className="block text-sm text-ink-muted">
                Für das, was in keiner Unfallstatistik steht.
              </span>
            </button>
          )}
        </div>
      </div>

      {/* Filters change what everything below says, so the result is announced
          rather than silently replaced under a screen reader. */}
      <div aria-live="polite" className="sr-only">
        {accidents.isSuccess
          ? `${summary.total} Unfälle im Umkreis von ${filters.radius} Metern`
          : ""}
      </div>

      <div className="mt-8 space-y-8">
        <KeyFigures
          summary={summary}
          radius={filters.radius}
          from={filters.from}
          to={filters.to}
        />
        <HotspotList hotspots={hotspots.data?.hotspots ?? []} onSelect={selectHotspot} />
        <AccidentTable accidents={accidentList} />
        <ReportList
          reports={reports.data?.reports ?? []}
          onConfirm={(report) => confirmReport.mutate(report)}
          confirmed={confirmed}
        />

        <section aria-labelledby="grenzen" className="rounded border border-line bg-white p-4">
          <h2 id="grenzen" className="text-lg font-semibold">
            Was diese Zahlen nicht hergeben
          </h2>
          <ul className="mt-2 list-disc space-y-1 pl-5 text-ink-muted">
            <li>
              Erfasst sind nur Unfälle <strong>mit Personenschaden</strong>. Beinahe-Unfälle und
              Sachschäden tauchen nicht auf.
            </li>
            <li>
              Die Koordinaten sind auf das Straßennetz gerastert und anonymisiert. Ein Punkt
              bezeichnet einen Straßenabschnitt, keine Stelle auf dem Asphalt.
            </li>
            <li>
              Es gibt keine Angaben zur Verkehrsmenge. Das sind absolute Häufigkeiten, keine
              Risikoraten.
            </li>
            <li>Dass nichts passiert ist, belegt nicht, dass eine Stelle sicher ist.</li>
          </ul>
        </section>
      </div>
    </div>
  );
}
