import type {
  AccidentList,
  Report,
  ReportList,
  ApiError,
  HotspotList,
  InfrastructureList,
  InstitutionDetail,
  InstitutionList,
} from "./types";

/** Requests go to the same origin; the reverse proxy routes /api. */
const base = "";

export class RequestFailed extends Error {
  readonly parameter?: string;
  constructor(message: string, parameter?: string) {
    super(message);
    this.name = "RequestFailed";
    this.parameter = parameter;
  }
}

async function failure(response: Response): Promise<RequestFailed> {
  // The API names the parameter at fault and why. Replacing that with a
  // generic message would throw away the only thing that helps.
  let body: ApiError | undefined;
  try {
    body = (await response.json()) as ApiError;
  } catch {
    body = undefined;
  }
  return new RequestFailed(
    body?.error ?? `Die Anfrage ist fehlgeschlagen (${response.status}).`,
    body?.parameter,
  );
}

async function send<T>(
  path: string,
  method: "POST",
  body?: unknown,
  token?: string,
): Promise<T> {
  const response = await fetch(`${base}${path}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) throw await failure(response);
  return (await response.json()) as T;
}

async function request<T>(
  path: string,
  params: Record<string, string | number | undefined>,
  token?: string,
) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") query.set(key, String(value));
  }
  const suffix = query.size > 0 ? `?${query}` : "";

  const response = await fetch(`${base}${path}${suffix}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (!response.ok) throw await failure(response);
  return (await response.json()) as T;
}

export type AccidentFilter = {
  radius: number;
  from?: number;
  to?: number;
  modes?: string;
};

export const api = {
  searchInstitutions: (q: string) =>
    request<InstitutionList>("/api/institutions", { q, limit: 25 }),

  institution: (id: number) => request<InstitutionDetail>(`/api/institutions/${id}`, {}),

  accidents: (id: number, filter: AccidentFilter) =>
    request<AccidentList>(`/api/institutions/${id}/accidents`, { ...filter }),

  hotspots: (id: number, radius: number) =>
    request<HotspotList>(`/api/institutions/${id}/hotspots`, { radius }),

  infrastructure: (id: number, radius: number) =>
    request<InfrastructureList>(`/api/institutions/${id}/infrastructure`, { radius }),

  reportsNear: (id: number, radius: number) =>
    request<ReportList>(`/api/institutions/${id}/reports`, { radius }),

  createReport: (submission: {
    lon: number;
    lat: number;
    category: string;
    description: string;
  }) => send<Report>("/api/reports", "POST", submission),

  confirmReport: (id: string) =>
    send<{ status: string }>(`/api/reports/${id}/confirm`, "POST"),

  moderationQueue: (token: string, status = "pending") =>
    request<ReportList>("/api/admin/reports", { status }, token),

  moderate: (token: string, id: string, status: "approved" | "rejected", note: string) =>
    send<{ status: string }>(`/api/admin/reports/${id}`, "POST", { status, note }, token),
};
