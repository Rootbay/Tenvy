export function isLikelyPrivateIp(ip: string): boolean {
        if (!ip) {
                return false;
        }

        const trimmed = ip.trim();
        if (!trimmed) {
                return false;
        }

        const normalized = trimmed
                .replace(/^\[(.*)]$/, '$1')
                .toLowerCase();

        if (
                normalized === '::1' ||
                normalized === '0:0:0:0:0:0:0:1' ||
                normalized === '::' ||
                normalized.startsWith('fe80:') ||
                normalized.startsWith('fc') ||
                normalized.startsWith('fd')
        ) {
                return true;
        }

        const ipv4Candidate = normalized.startsWith('::ffff:')
                ? normalized.slice('::ffff:'.length)
                : normalized;

        if (ipv4Candidate === '0.0.0.0') {
                return true;
        }

        return (
                ipv4Candidate.startsWith('10.') ||
                ipv4Candidate.startsWith('192.168.') ||
                /^172\.(1[6-9]|2\d|3[0-1])\./.test(ipv4Candidate) ||
                ipv4Candidate.startsWith('127.')
        );
}
