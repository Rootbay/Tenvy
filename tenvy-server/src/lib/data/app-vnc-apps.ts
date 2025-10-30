import type { AppVncApplicationDescriptor } from '$lib/types/app-vnc';

const applications: readonly AppVncApplicationDescriptor[] = [
	{
		id: 'browser.chromium',
		name: 'Chromium',
		summary: 'Open-source Chromium browser profile optimised for covert web operations.',
		category: 'Browser',
		platforms: ['windows', 'linux', 'macos'],
		windowTitleHint: 'Chromium',
		executable: {
			windows: 'C:\\Program Files\\Chromium\\Application\\chrome.exe',
			linux: '/usr/bin/chromium-browser',
			macos: '/Applications/Chromium.app/Contents/MacOS/Chromium'
		},
		virtualization: {
			profileSeeds: {
				windows: 'C:\\ProgramData\\Tenvy\\appvnc\\chromium-profile',
				linux: '/opt/tenvy/appvnc/chromium-profile',
				macos: '/Library/Application Support/Tenvy/appvnc/chromium-profile'
			},
			dataRoots: {
				windows: 'C:\\ProgramData\\Tenvy\\appvnc\\chromium-data',
				linux: '/opt/tenvy/appvnc/chromium-data',
				macos: '/Library/Application Support/Tenvy/appvnc/chromium-data'
			}
		}
	},
	{
		id: 'browser.firefox',
		name: 'Firefox',
		summary: 'Mozilla Firefox profile with isolated session storage for investigative browsing.',
		category: 'Browser',
		platforms: ['windows', 'linux', 'macos'],
		windowTitleHint: 'Mozilla Firefox',
		executable: {
			windows: 'C:\\Program Files\\Mozilla Firefox\\firefox.exe',
			linux: '/usr/bin/firefox',
			macos: '/Applications/Firefox.app/Contents/MacOS/firefox'
		},
		virtualization: {
			profileSeeds: {
				windows: 'C:\\ProgramData\\Tenvy\\appvnc\\firefox-profile',
				linux: '/opt/tenvy/appvnc/firefox-profile',
				macos: '/Library/Application Support/Tenvy/appvnc/firefox-profile'
			},
			dataRoots: {
				windows: 'C:\\ProgramData\\Tenvy\\appvnc\\firefox-data',
				linux: '/opt/tenvy/appvnc/firefox-data',
				macos: '/Library/Application Support/Tenvy/appvnc/firefox-data'
			}
		}
	},
	{
		id: 'comms.discord',
		name: 'Discord',
		summary: 'Discord desktop client wrapped for controlled communications.',
		category: 'Communication',
		platforms: ['windows', 'macos'],
		windowTitleHint: 'Discord',
		executable: {
			windows: 'C:\\Users\\%USERNAME%\\AppData\\Local\\Discord\\Update.exe',
			macos: '/Applications/Discord.app/Contents/MacOS/Discord'
		},
		virtualization: {
			profileSeeds: {
				windows: 'C:\\ProgramData\\Tenvy\\appvnc\\discord-profile',
				macos: '/Library/Application Support/Tenvy/appvnc/discord-profile'
			},
			dataRoots: {
				windows: 'C:\\ProgramData\\Tenvy\\appvnc\\discord-data',
				macos: '/Library/Application Support/Tenvy/appvnc/discord-data'
			},
			environment: {
				windows: {
					NODE_ENV: 'production',
					DISCORD_SKIP_HOST_UPDATE: '1'
				}
			}
		}
	},
	{
		id: 'comms.telegram',
		name: 'Telegram',
		summary: 'Telegram Desktop for rapid operator messaging.',
		category: 'Communication',
		platforms: ['windows', 'linux', 'macos'],
		windowTitleHint: 'Telegram',
		executable: {
			windows: 'C:\\Users\\%USERNAME%\\AppData\\Roaming\\Telegram Desktop\\Telegram.exe',
			linux: '/usr/bin/telegram-desktop',
			macos: '/Applications/Telegram.app/Contents/MacOS/Telegram'
		},
		virtualization: {
			profileSeeds: {
				windows: 'C:\\ProgramData\\Tenvy\\appvnc\\telegram-profile',
				linux: '/opt/tenvy/appvnc/telegram-profile',
				macos: '/Library/Application Support/Tenvy/appvnc/telegram-profile'
			},
			dataRoots: {
				windows: 'C:\\ProgramData\\Tenvy\\appvnc\\telegram-data',
				linux: '/opt/tenvy/appvnc/telegram-data',
				macos: '/Library/Application Support/Tenvy/appvnc/telegram-data'
			}
		}
	}
] as const;

function cloneApplication(app: AppVncApplicationDescriptor): AppVncApplicationDescriptor {
	return {
		...app,
		platforms: [...app.platforms],
		executable: app.executable ? { ...app.executable } : undefined,
		virtualization: app.virtualization
			? {
					profileSeeds: app.virtualization.profileSeeds
						? { ...app.virtualization.profileSeeds }
						: undefined,
					dataRoots: app.virtualization.dataRoots ? { ...app.virtualization.dataRoots } : undefined,
					environment: app.virtualization.environment
						? Object.fromEntries(
								Object.entries(app.virtualization.environment).map(([platform, values]) => [
									platform,
									{ ...values }
								])
							)
						: undefined
				}
			: undefined
	};
}

export const appVncApplications: readonly AppVncApplicationDescriptor[] = applications.map((app) =>
	cloneApplication(app)
);

export function listAppVncApplications(): AppVncApplicationDescriptor[] {
	return applications.map((app) => cloneApplication(app));
}

export function findAppVncApplication(
	id: string | null | undefined
): AppVncApplicationDescriptor | null {
	if (!id) {
		return null;
	}
	const target = applications.find((app) => app.id === id);
	return target ? cloneApplication(target) : null;
}
