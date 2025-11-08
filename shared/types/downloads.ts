/**
 * Entry describing a downloadable artefact that can be delivered to an agent.
 */
export interface DownloadCatalogueEntry {
  /** Unique identifier for the download entry. */
  id: string;
  /** Human readable name to show in the UI. */
  displayName: string;
  /** Optional description surfaced alongside the entry. */
  description?: string;
  /** Optional version string for the artefact. */
  version?: string;
  /** Optional executable name expected to be present after download. */
  executable?: string;
  /** Optional filesystem path to install or reference the artefact. */
  path?: string;
  /** Optional integrity hash for the download. */
  hash?: string;
  /** Optional size of the download in bytes. */
  sizeBytes?: number;
  /** Optional tags used for filtering and organisation. */
  tags?: string[];
}

export type DownloadCatalogue = DownloadCatalogueEntry[];

export interface DownloadCatalogueResponse {
  downloads: DownloadCatalogue;
}
