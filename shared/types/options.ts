export interface OptionsScriptFile {
  name: string;
  size: number;
  type: string;
  path: string;
  checksum: string;
}

export interface OptionsScriptConfig {
  file?: OptionsScriptFile | null;
  mode?: string;
  loop?: boolean;
  delaySeconds?: number;
}

export interface OptionsScriptRuntimeState {
  status?: string;
  active?: boolean;
  lastStartedAt?: string | null;
  lastCompletedAt?: string | null;
  lastExitCode?: number;
  hasExitCode?: boolean;
  lastError?: string;
  runs?: number;
}

export interface OptionsState {
  defenderExclusion?: boolean;
  windowsUpdate?: boolean;
  visualDistortion?: string;
  screenOrientation?: string;
  wallpaperMode?: string;
  cursorBehavior?: string;
  keyboardMode?: string;
  soundPlayback?: boolean;
  soundVolume?: number;
  script?: OptionsScriptConfig | null;
  scriptRuntime?: OptionsScriptRuntimeState | null;
  fakeEventMode?: string;
  speechSpam?: boolean;
  autoMinimize?: boolean;
}

export interface AgentOptionsResponse {
  state: OptionsState | null;
}

export interface AgentOptionsUpdateRequest {
  state: OptionsState | null;
}

export interface AgentOptionsUpdateResponse {
  state: OptionsState | null;
}
