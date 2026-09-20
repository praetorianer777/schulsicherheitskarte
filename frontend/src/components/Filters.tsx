import { useId } from "react";

import type { LayerVisibility } from "./MapView";

export type FilterState = {
  radius: number;
  from: number;
  to: number;
  onlyVulnerable: boolean;
};

export const radiusOptions = [250, 500, 1000] as const;

type Props = {
  value: FilterState;
  onChange: (next: FilterState) => void;
  layers: LayerVisibility;
  onLayersChange: (next: LayerVisibility) => void;
  years: { first: number; last: number };
};

export default function Filters({ value, onChange, layers, onLayersChange, years }: Props) {
  const radiusName = useId();
  const fromId = useId();

  return (
    <div className="rounded border border-line bg-white p-4">
      <h2 className="text-sm font-semibold">Filter</h2>

      <fieldset className="mt-3">
        <legend className="text-sm font-medium">Umkreis</legend>
        <div className="mt-1 flex flex-wrap gap-4">
          {radiusOptions.map((radius) => (
            <label key={radius} className="flex items-center gap-2">
              <input
                type="radio"
                name={radiusName}
                value={radius}
                checked={value.radius === radius}
                onChange={() => onChange({ ...value, radius })}
              />
              {radius} m
            </label>
          ))}
        </div>
      </fieldset>

      <div className="mt-3">
        <label htmlFor={fromId} className="block text-sm font-medium">
          Ab Berichtsjahr
        </label>
        <select
          id={fromId}
          value={value.from}
          onChange={(event) => onChange({ ...value, from: Number(event.target.value) })}
          className="mt-1 rounded border border-line bg-white px-2 py-1"
        >
          {yearRange(years.first, years.last).map((year) => (
            <option key={year} value={year}>
              {year}
            </option>
          ))}
        </select>
      </div>

      <div className="mt-3">
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={value.onlyVulnerable}
            onChange={(event) => onChange({ ...value, onlyVulnerable: event.target.checked })}
          />
          <span>
            Nur Unfälle mit Fuß- oder Radbeteiligung
            <span className="block text-sm text-ink-muted">
              Der Blick auf den Schulweg: wer zu Fuß oder mit dem Rad unterwegs war.
            </span>
          </span>
        </label>
      </div>

      <fieldset className="mt-4 border-t border-line pt-3">
        <legend className="text-sm font-medium">Ebenen</legend>
        <div className="mt-1 space-y-1">
          <LayerToggle
            label="Unfälle"
            checked={layers.accidents}
            onChange={(accidents) => onLayersChange({ ...layers, accidents })}
          />
          <LayerToggle
            label="Unfallschwerpunkte"
            checked={layers.hotspots}
            onChange={(hotspots) => onLayersChange({ ...layers, hotspots })}
          />
          <LayerToggle
            label="Querungen, Ampeln, Tempolimits"
            checked={layers.infrastructure}
            onChange={(infrastructure) => onLayersChange({ ...layers, infrastructure })}
          />
        </div>
      </fieldset>
    </div>
  );
}

function LayerToggle({
  label,
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <label className="flex items-center gap-2">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      {label}
    </label>
  );
}

function yearRange(first: number, last: number) {
  const years: number[] = [];
  for (let year = first; year <= last; year++) years.push(year);
  return years;
}
