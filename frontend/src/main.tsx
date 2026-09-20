import { config as maplibreConfig } from "maplibre-gl";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter } from "react-router-dom";

import App from "./App";
import "./index.css";
import "maplibre-gl/dist/maplibre-gl.css";
import maplibreWorkerURL from "maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url";

// MapLibre derives the worker URL at runtime from import.meta.url. Rollup never
// sees that string, so it emits no asset and the built bundle asks for a file
// that does not exist — the map then stays empty in every image, while it works
// in development, where the path resolves into node_modules.
//
// It is imported as a worker rather than as a plain asset: the file imports
// MapLibre's shared chunk, and copying it verbatim would only move the missing
// file one level down.
maplibreConfig.WORKER_URL = maplibreWorkerURL;

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // The underlying data changes once a year at most; refetching on every
      // window focus would only produce flicker.
      staleTime: 5 * 60 * 1000,
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
