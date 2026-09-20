import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import ReportForm from "./ReportForm";
import { axe, renderApp } from "../test/render";
import { describeViolations } from "../test/axe";
import { school } from "../test/fixtures";

function setup(overrides: Partial<Parameters<typeof ReportForm>[0]> = {}) {
  const onSubmit = vi.fn();
  const onPointChange = vi.fn();
  const onCancel = vi.fn();
  const result = renderApp(
    <ReportForm
      institution={school}
      point={{ lon: school.lon, lat: school.lat }}
      onPointChange={onPointChange}
      onSubmit={onSubmit}
      onCancel={onCancel}
      {...overrides}
    />,
  );
  return { ...result, onSubmit, onPointChange, onCancel };
}

describe("Meldeformular", () => {
  it("schickt Ort, Kategorie und Beschreibung ab", async () => {
    const user = userEvent.setup();
    const { onSubmit } = setup();

    await user.selectOptions(screen.getByLabelText("Worum geht es?"), "speeding");
    await user.type(screen.getByLabelText(/Beschreibung/), "Morgens rasen alle.");
    await user.click(screen.getByRole("button", { name: "Meldung abschicken" }));

    expect(onSubmit).toHaveBeenCalledWith({
      lon: school.lon,
      lat: school.lat,
      category: "speeding",
      description: "Morgens rasen alle.",
    });
  });

  // Placing a point by clicking the map is a mouse gesture. Without an
  // alternative, the reporting feature would be closed to keyboard users.
  it("lässt den Ort ohne Maus verschieben", async () => {
    const user = userEvent.setup();
    const { onPointChange } = setup();

    await user.click(screen.getByRole("button", { name: "nach Norden" }));

    expect(onPointChange).toHaveBeenCalledTimes(1);
    const moved = onPointChange.mock.calls[0][0];
    expect(moved.lat).toBeGreaterThan(school.lat);
    // 25 m north, within a metre.
    expect((moved.lat - school.lat) * 111_320).toBeCloseTo(25, 0);
  });

  // Somebody who cannot see the marker move still has to know where it went.
  it("beschreibt den gewählten Ort in Worten", () => {
    setup({ point: { lon: school.lon, lat: school.lat + 0.0009 } });
    expect(screen.getByText(/100 m nördlich der Einrichtung/)).toBeInTheDocument();
  });

  // If people do not know it is moderated, the first reaction to an invisible
  // report is that the site is broken.
  it("sagt, dass die Meldung erst gesichtet wird", () => {
    setup();
    expect(screen.getByText(/vor der Veröffentlichung gesichtet/)).toBeInTheDocument();
  });

  it("nennt, was gespeichert wird und was nicht", () => {
    setup();
    expect(screen.getByText(/kein Name und keine Adresse/)).toBeInTheDocument();
  });

  it("zeigt die Begründung der API bei einer Ablehnung", () => {
    setup({ error: "Es wurden bereits mehrere Meldungen von diesem Anschluss abgeschickt." });
    expect(screen.getByRole("alert")).toHaveTextContent(/bereits mehrere Meldungen/);
  });

  it("bestätigt den Eingang und erklärt die Moderation", () => {
    setup({ submitted: true });
    expect(screen.getByRole("status")).toHaveTextContent(/Danke, die Meldung ist eingegangen/);
    expect(screen.getByText(/erst danach auf der Karte/)).toBeInTheDocument();
  });

  it("hat keine Barrierefreiheitsverstöße", async () => {
    const { container } = setup();
    expect(describeViolations(await axe(container))).toBe("");
  });
});
