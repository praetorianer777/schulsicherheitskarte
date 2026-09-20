import { useId, useState } from "react";

import type { Institution, ReportCategory } from "../api/types";
import { describeOffset, reportCategoryLabel } from "../lib/format";

export type ReportDraft = {
  lon: number;
  lat: number;
  category: ReportCategory;
  description: string;
};

type Props = {
  institution: Institution;
  point: { lon: number; lat: number };
  onPointChange: (point: { lon: number; lat: number }) => void;
  onSubmit: (draft: ReportDraft) => void;
  onCancel: () => void;
  busy?: boolean;
  error?: string | null;
  submitted?: boolean;
};

/** One nudge, in metres. Small enough to be precise, large enough to matter. */
const step = 25;

const categories = Object.keys(reportCategoryLabel) as ReportCategory[];

export default function ReportForm({
  institution,
  point,
  onPointChange,
  onSubmit,
  onCancel,
  busy,
  error,
  submitted,
}: Props) {
  const categoryId = useId();
  const descriptionId = useId();
  const [category, setCategory] = useState<ReportCategory>("crossing_unsafe");
  const [description, setDescription] = useState("");

  const move = (northMetres: number, eastMetres: number) => {
    const lat = point.lat + northMetres / 111_320;
    const lon = point.lon + eastMetres / (111_320 * Math.cos((point.lat * Math.PI) / 180));
    onPointChange({ lon, lat });
  };

  if (submitted) {
    return (
      <div className="rounded border border-line bg-white p-4" role="status">
        <h3 className="font-semibold">Danke, die Meldung ist eingegangen.</h3>
        <p className="mt-2 text-ink-muted">
          Sie wird zuerst gesichtet und erscheint erst danach auf der Karte. Das schützt die
          Karte davor, als Beschwerdewand missbraucht zu werden.
        </p>
        <button type="button" onClick={onCancel} className="mt-3 rounded border border-line px-3 py-2">
          Schließen
        </button>
      </div>
    );
  }

  return (
    <form
      className="rounded border border-line bg-white p-4"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit({ ...point, category, description: description.trim() });
      }}
    >
      <h3 className="font-semibold">Gefahrenstelle melden</h3>
      <p className="mt-1 text-sm text-ink-muted">
        Beinahe-Unfälle tauchen in keiner Unfallstatistik auf. Genau dafür ist das hier.
      </p>

      <fieldset className="mt-4">
        <legend className="text-sm font-medium">Ort</legend>
        <p className="mt-1 text-sm" aria-live="polite">
          Gewählt: {describeOffset(point, institution)}
        </p>
        <p className="mt-1 text-sm text-ink-muted">
          Auf der Karte klicken, oder hier in Schritten von {step} m verschieben:
        </p>
        <div className="mt-2 grid w-40 grid-cols-3 gap-1">
          <span />
          <NudgeButton label="nach Norden" onClick={() => move(step, 0)}>
            ↑
          </NudgeButton>
          <span />
          <NudgeButton label="nach Westen" onClick={() => move(0, -step)}>
            ←
          </NudgeButton>
          <NudgeButton label="zurück zur Einrichtung" onClick={() => onPointChange(institution)}>
            ⌂
          </NudgeButton>
          <NudgeButton label="nach Osten" onClick={() => move(0, step)}>
            →
          </NudgeButton>
          <span />
          <NudgeButton label="nach Süden" onClick={() => move(-step, 0)}>
            ↓
          </NudgeButton>
          <span />
        </div>
      </fieldset>

      <div className="mt-4">
        <label htmlFor={categoryId} className="block text-sm font-medium">
          Worum geht es?
        </label>
        <select
          id={categoryId}
          value={category}
          onChange={(event) => setCategory(event.target.value as ReportCategory)}
          className="mt-1 w-full rounded border border-line bg-white px-2 py-2"
        >
          {categories.map((value) => (
            <option key={value} value={value}>
              {reportCategoryLabel[value]}
            </option>
          ))}
        </select>
      </div>

      <div className="mt-4">
        <label htmlFor={descriptionId} className="block text-sm font-medium">
          Beschreibung <span className="font-normal text-ink-muted">(optional)</span>
        </label>
        <textarea
          id={descriptionId}
          value={description}
          maxLength={1000}
          rows={4}
          onChange={(event) => setDescription(event.target.value)}
          className="mt-1 w-full rounded border border-line bg-white px-2 py-2"
          placeholder="Was passiert dort, und wann?"
        />
        <p className="text-sm text-ink-muted">
          Bitte keine Namen, Kennzeichen oder andere Angaben zu einzelnen Personen.
        </p>
      </div>

      {error && (
        <p role="alert" className="mt-3 text-critical">
          {error}
        </p>
      )}

      <div className="mt-4 flex gap-2">
        <button
          type="submit"
          disabled={busy}
          className="rounded bg-accent px-4 py-2 font-medium text-white disabled:opacity-60"
        >
          Meldung abschicken
        </button>
        <button type="button" onClick={onCancel} className="rounded border border-line px-4 py-2">
          Abbrechen
        </button>
      </div>

      <p className="mt-3 text-sm text-ink-muted">
        Die Meldung wird vor der Veröffentlichung gesichtet. Gespeichert wird kein Name und
        keine Adresse — nur ein gesalzener Prüfwert, damit nicht eine Person beliebig viele
        Meldungen abgeben kann.
      </p>
    </form>
  );
}

function NudgeButton({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      className="rounded border border-line px-2 py-1 text-lg"
    >
      <span aria-hidden="true">{children}</span>
    </button>
  );
}
