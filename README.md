# 🚸 Schulweg-Sicherheitskarte

Accident hotspots around every school and kindergarten in Germany, built from open
data — so parent representatives can argue for a crosswalk or a 30 km/h zone with
numbers instead of anecdotes.

> The user interface is German, because its users are German parents, school
> administrations and road traffic authorities. Everything else in this repository —
> code, comments, commits, issues, documentation — is English.

## Why

Since the 2024 amendment of the German road traffic regulations (StVO), municipalities
can order 30 km/h speed limits in front of schools and kindergartens far more easily.
What is usually missing in the individual case is evidence. It exists, but it is
scattered:

- The **Unfallatlas** of the German statistical offices publishes every road accident
  involving personal injury as an open, georeferenced dataset — including whether a
  pedestrian or a cyclist was involved.
- **OpenStreetMap** knows where the schools, kindergartens, crossings, traffic signals
  and speed limits are.

Nobody has put the two together per school. This project does, adds a crowdsourcing
layer for near misses that never reach any statistic, and produces a printable
fact sheet for the next road safety inspection.

## Data sources

| Source | What | Licence |
|---|---|---|
| [Unfallatlas](https://unfallatlas.statistikportal.de/), reporting years 2016–2025 | accidents involving personal injury, point geometry, pedestrian/cyclist involvement | `dl-de/by-2-0` |
| [OpenStreetMap](https://www.openstreetmap.org/) via Overpass API | schools, kindergartens, crossings, traffic signals, traffic calming, speed limits | ODbL, attribution “© OpenStreetMap contributors” |

### What the data cannot tell you

Stated on every fact sheet, because a claim that overreaches is taken apart at the
first road safety inspection:

- Only accidents **with personal injury** are recorded. Near misses, property damage
  and everyday intimidation are invisible — that is what the reporting feature is for.
- Coordinates are snapped to the road network and anonymised; a point marks a road
  section, not a spot on the asphalt.
- There is no traffic volume data, so these are **absolute frequencies, not risk
  rates**. A quiet street with one accident is not automatically safer than a busy one
  with three.
- The absence of accidents is not evidence of safety.

## Accessibility

Non-negotiable, not a later clean-up pass. Target is **WCAG 2.2 level AA**:

- every map has an equivalent list view — map content is never the only way to reach
  information
- fully keyboard operable, visible focus, no keyboard traps
- severity is never encoded by colour alone; contrast ratios are checked, not guessed
- correct document language, heading structure and form labels, live regions for
  asynchronous results
- `prefers-reduced-motion` is honoured for map animations
- automated `axe-core` checks run in the test suite; they catch regressions, they do
  not replace manual keyboard and screen reader testing

## Stack

- **Backend** — Go, PostgreSQL + PostGIS
- **Frontend** — React, TypeScript, TailwindCSS, MapLibre GL
- **Deployment** — Docker Compose, self-hosted

## Development

```bash
./run-tests.sh          # the one entry point: shell, Go, frontend, end-to-end
```

Every change starts from a GitHub issue and lives on a branch
`<type>/<issue>-<slug>`. `.claude/hooks/branch-guard.sh` enforces that: it refuses
edits, commits and pushes outside an issue branch, refuses pushes to `main`, and runs
`./run-tests.sh` before letting any push through.

## Licence

[AGPL-3.0](LICENSE). This is a service people host rather than distribute, so the
network clause is the point: anyone running a modified version publicly has to publish
their changes.
