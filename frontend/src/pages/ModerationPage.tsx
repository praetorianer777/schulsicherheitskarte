import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";

import { api } from "../api/client";
import type { Report } from "../api/types";
import { reportCategoryLabel } from "../lib/format";

/**
 * The moderation queue. The token is held in this component only — putting it
 * in the URL or in storage would leave it lying around on a shared machine.
 */
export default function ModerationPage() {
  const tokenId = useId();
  const [tokenInput, setTokenInput] = useState("");
  const [token, setToken] = useState("");
  const client = useQueryClient();

  const queue = useQuery({
    queryKey: ["moderation", token],
    queryFn: () => api.moderationQueue(token),
    enabled: token !== "",
  });

  const decide = useMutation({
    mutationFn: ({ report, status, note }: { report: Report; status: "approved" | "rejected"; note: string }) =>
      api.moderate(token, report.id, status, note),
    onSuccess: () => client.invalidateQueries({ queryKey: ["moderation", token] }),
  });

  if (token === "") {
    return (
      <div className="mx-auto max-w-md px-4 py-10">
        <h1 className="text-2xl font-semibold">Moderation</h1>
        <form
          className="mt-4"
          onSubmit={(event) => {
            event.preventDefault();
            setToken(tokenInput.trim());
          }}
        >
          <label htmlFor={tokenId} className="block text-sm font-medium">
            Moderations-Token
          </label>
          <input
            id={tokenId}
            type="password"
            value={tokenInput}
            autoComplete="off"
            onChange={(event) => setTokenInput(event.target.value)}
            className="mt-1 w-full rounded border border-line bg-white px-3 py-2"
          />
          <button type="submit" className="mt-3 rounded bg-accent px-4 py-2 font-medium text-white">
            Warteschlange öffnen
          </button>
        </form>
      </div>
    );
  }

  const waiting = queue.data?.reports ?? [];

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      <h1 className="text-2xl font-semibold">Moderation</h1>

      <div aria-live="polite" className="mt-4">
        {queue.isFetching && <p>Warteschlange wird geladen …</p>}
        {queue.isError && (
          <p className="text-critical">
            {(queue.error as Error).message}{" "}
            <button type="button" className="underline" onClick={() => setToken("")}>
              Anderes Token eingeben
            </button>
          </p>
        )}
        {queue.isSuccess && waiting.length === 0 && <p>Nichts zu tun — die Warteschlange ist leer.</p>}
      </div>

      <ul className="mt-4 space-y-4">
        {waiting.map((report) => (
          <QueueEntry
            key={report.id}
            report={report}
            busy={decide.isPending}
            onDecide={(status, note) => decide.mutate({ report, status, note })}
          />
        ))}
      </ul>
    </div>
  );
}

function QueueEntry({
  report,
  onDecide,
  busy,
}: {
  report: Report;
  onDecide: (status: "approved" | "rejected", note: string) => void;
  busy: boolean;
}) {
  const noteId = useId();
  const [note, setNote] = useState("");

  return (
    <li className="rounded border border-line bg-white p-4">
      <p className="font-medium">{reportCategoryLabel[report.category] ?? report.category}</p>
      {report.description && <p className="mt-1">{report.description}</p>}
      <p className="mt-1 text-sm text-ink-muted">
        {new Date(report.createdAt).toLocaleString("de-DE")} · {report.lat.toFixed(5)},{" "}
        {report.lon.toFixed(5)}
      </p>

      <label htmlFor={noteId} className="mt-3 block text-sm font-medium">
        Notiz <span className="font-normal text-ink-muted">(optional, nicht öffentlich)</span>
      </label>
      <input
        id={noteId}
        value={note}
        onChange={(event) => setNote(event.target.value)}
        className="mt-1 w-full rounded border border-line bg-white px-2 py-1"
      />

      <div className="mt-3 flex gap-2">
        <button
          type="button"
          disabled={busy}
          onClick={() => onDecide("approved", note)}
          className="rounded bg-accent px-3 py-2 text-white disabled:opacity-60"
        >
          Freigeben
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={() => onDecide("rejected", note)}
          className="rounded border border-line px-3 py-2 disabled:opacity-60"
        >
          Ablehnen
        </button>
      </div>
    </li>
  );
}
