import { browser } from '$app/environment';

const LOCAL_PORT_KEY = 'tenvy.selectedPorts';
const LOCAL_REMEMBER_KEY = 'tenvy.rememberPorts';
const SESSION_PORT_KEY = 'tenvy.sessionPorts';

const PORT_MIN = 1;
const PORT_MAX = 65_535;

type ParseResult = { ok: true; ports: number[] } | { ok: false; error: string };

function normalizePorts(values: number[]) {
        const unique = Array.from(new Set(values));
        unique.sort((a, b) => a - b);
        return unique;
}

export function formatPortSummary(ports: number[]): string {
        return ports.length > 0 ? ports.join(', ') : '';
}

export function parsePortInput(raw: string): ParseResult {
        const tokens = raw
                .split(/[\s,]+/)
                .map((token) => token.trim())
                .filter(Boolean);

        if (tokens.length === 0) {
                return { ok: false, error: 'Enter at least one port.' };
        }

        const values: number[] = [];

        for (const token of tokens) {
                const port = Number(token);

                if (!Number.isInteger(port) || port < PORT_MIN || port > PORT_MAX) {
                        return {
                                ok: false,
                                error: `Port "${token}" is not valid. Use values between ${PORT_MIN.toLocaleString()} and ${PORT_MAX.toLocaleString()}.`
                        };
                }

                values.push(port);
        }

        return { ok: true, ports: normalizePorts(values) };
}

function parseStoredPorts(raw: string | null): number[] | null {
        if (!raw) {
                return null;
        }

        try {
                const parsed = JSON.parse(raw) as unknown;
                if (!Array.isArray(parsed)) {
                        return null;
                }

                const ports = parsed
                        .map((value) => Number(value))
                        .filter((value) => Number.isInteger(value) && value >= PORT_MIN && value <= PORT_MAX);

                if (ports.length !== parsed.length || ports.length === 0) {
                        return null;
                }

                return normalizePorts(ports);
        } catch (error) {
                console.error('Failed to parse stored RAT port preferences', error);
                return null;
        }
}

export type StoredPortPreference = { ports: number[]; remember: boolean };

export function loadStoredPorts(): StoredPortPreference | null {
        if (!browser) {
                return null;
        }

        try {
                const rememberFlag = window.localStorage.getItem(LOCAL_REMEMBER_KEY);

                if (rememberFlag === 'true') {
                        const storedPorts = parseStoredPorts(window.localStorage.getItem(LOCAL_PORT_KEY));
                        if (storedPorts) {
                                return { ports: storedPorts, remember: true };
                        }

                        window.localStorage.removeItem(LOCAL_PORT_KEY);
                        window.localStorage.removeItem(LOCAL_REMEMBER_KEY);
                }

                const sessionPorts = parseStoredPorts(window.sessionStorage.getItem(SESSION_PORT_KEY));
                if (sessionPorts) {
                        return { ports: sessionPorts, remember: false };
                }

                window.sessionStorage.removeItem(SESSION_PORT_KEY);
        } catch (error) {
                console.error('Failed to restore RAT port preferences', error);
        }

        return null;
}

export function persistPortSelection(ports: number[], remember: boolean) {
        if (!browser) {
                return;
        }

        try {
                if (ports.length === 0) {
                        clearStoredPorts();
                        return;
                }

                window.sessionStorage.setItem(SESSION_PORT_KEY, JSON.stringify(ports));

                if (remember) {
                        window.localStorage.setItem(LOCAL_PORT_KEY, JSON.stringify(ports));
                        window.localStorage.setItem(LOCAL_REMEMBER_KEY, 'true');
                } else {
                        window.localStorage.removeItem(LOCAL_PORT_KEY);
                        window.localStorage.removeItem(LOCAL_REMEMBER_KEY);
                }
        } catch (error) {
                console.error('Failed to persist RAT port preferences', error);
        }
}

export function clearStoredPorts() {
        if (!browser) {
                return;
        }

        try {
                window.sessionStorage.removeItem(SESSION_PORT_KEY);
                window.localStorage.removeItem(LOCAL_PORT_KEY);
                window.localStorage.removeItem(LOCAL_REMEMBER_KEY);
        } catch (error) {
                console.error('Failed to clear RAT port preferences', error);
        }
}
