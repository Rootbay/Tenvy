export function formatMetric(value: number | null, suffix: string, digits = 1) {
        if (value === null || Number.isNaN(value)) {
                return `-- ${suffix}`;
        }
        return `${value.toFixed(digits)} ${suffix}`;
}

export function formatPercent(value: number | null) {
        if (value === null || Number.isNaN(value)) {
                return '-- %';
        }
        return `${Math.round(value)}%`;
}

export function formatResolution(width: number | null, height: number | null) {
        if (width === null || height === null || Number.isNaN(width) || Number.isNaN(height)) {
                return '--';
        }
        return `${width}×${height}`;
}

export function formatLatency(value: number | null) {
        if (value === null || Number.isNaN(value)) {
                return '-- ms';
        }
        if (value >= 1000) {
                return `${(value / 1000).toFixed(1)} s`;
        }
        return `${Math.round(value)} ms`;
}

export function formatTimestamp(value: string | null | undefined) {
        if (!value) return '—';
        const parsed = new Date(value);
        if (Number.isNaN(parsed.getTime())) {
                return value;
        }
        return parsed.toLocaleTimeString();
}
