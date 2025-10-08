export function splitCommandLine(input: string): string[] {
        const tokens: string[] = [];
        let current = '';
        let quote: '"' | "'" | null = null;
        let escape = false;

        for (const char of input.trim()) {
                if (escape) {
                        current += char;
                        escape = false;
                        continue;
                }

                if (char === '\\' && quote !== "'") {
                        escape = true;
                        continue;
                }

                if (char === '"' || char === "'") {
                        if (quote === char) {
                                quote = null;
                                continue;
                        }
                        if (!quote) {
                                quote = char;
                                continue;
                        }
                }

                if (!quote && /\s/.test(char)) {
                        if (current) {
                                tokens.push(current);
                                current = '';
                        }
                        continue;
                }

                current += char;
        }

        if (current) {
                tokens.push(current);
        }

        return tokens;
}
