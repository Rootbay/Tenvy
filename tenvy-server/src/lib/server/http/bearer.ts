export function getBearerToken(header: string | null): string | undefined {
        if (!header) {
                return undefined;
        }

        const match = header.match(/^Bearer\s+(.+)$/i);
        return match?.[1]?.trim();
}
