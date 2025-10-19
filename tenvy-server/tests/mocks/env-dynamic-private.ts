export const env: Record<string, string | undefined> = new Proxy(
	{},
	{
		get: (_target, property: string) => {
			if (property === 'DATABASE_URL' && !process.env.DATABASE_URL) {
				process.env.DATABASE_URL = ':memory:';
			}
			return process.env[property];
		},
		has: (_target, property: string) => property in process.env
	}
);
