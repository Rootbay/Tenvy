export function splitCommandLine(input: string): string[] {
        const tokens: string[] = [];
        let current = '';
        let quote: '"' | "'" | null = null;
        let escape = false;
        let inToken = false;

        for (const char of input.trim()) {
                if (escape) {
                        current += char;
                        escape = false;
                        inToken = true;
                        continue;
                }

                if (char === '\\' && quote !== "'") {
                        escape = true;
                        inToken = true;
                        continue;
                }

                if (char === '"' || char === "'") {
                        if (quote === char) {
                                quote = null;
                                inToken = true;
                                continue;
                        }
                        if (!quote) {
                                quote = char;
                                inToken = true;
                                continue;
                        }
                }

                if (!quote && /\s/.test(char)) {
                        if (inToken) {
                                tokens.push(current);
                                current = '';
                                inToken = false;
                        }
                        continue;
                }

                current += char;
                inToken = true;
        }

        if (inToken) {
                tokens.push(current);
        }

        return tokens;
}
