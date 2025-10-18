import type { AppVncApplicationDescriptor } from '$lib/types/app-vnc';

const applications: readonly AppVncApplicationDescriptor[] = [
        {
                id: 'browser.chromium',
                name: 'Chromium',
                summary: 'Open-source Chromium browser profile optimised for covert web operations.',
                category: 'Browser',
                platforms: ['windows', 'linux'],
                windowTitleHint: 'Chromium',
                executable: {
                        windows: 'C:\\Program Files\\Chromium\\Application\\chrome.exe',
                        linux: '/usr/bin/chromium-browser'
                }
        },
        {
                id: 'browser.firefox',
                name: 'Firefox',
                summary: 'Mozilla Firefox profile with isolated session storage for investigative browsing.',
                category: 'Browser',
                platforms: ['windows', 'linux'],
                windowTitleHint: 'Mozilla Firefox',
                executable: {
                        windows: 'C:\\Program Files\\Mozilla Firefox\\firefox.exe',
                        linux: '/usr/bin/firefox'
                }
        },
        {
                id: 'comms.discord',
                name: 'Discord',
                summary: 'Discord desktop client wrapped for controlled communications.',
                category: 'Communication',
                platforms: ['windows'],
                windowTitleHint: 'Discord',
                executable: {
                        windows: 'C:\\Users\\%USERNAME%\\AppData\\Local\\Discord\\Update.exe'
                }
        },
        {
                id: 'comms.telegram',
                name: 'Telegram',
                summary: 'Telegram Desktop for rapid operator messaging.',
                category: 'Communication',
                platforms: ['windows', 'linux'],
                windowTitleHint: 'Telegram',
                executable: {
                        windows: 'C:\\Users\\%USERNAME%\\AppData\\Roaming\\Telegram Desktop\\Telegram.exe',
                        linux: '/usr/bin/telegram-desktop'
                }
        }
] as const;

function cloneApplication(app: AppVncApplicationDescriptor): AppVncApplicationDescriptor {
        return {
                ...app,
                platforms: [...app.platforms],
                executable: app.executable ? { ...app.executable } : undefined
        };
}

export const appVncApplications: readonly AppVncApplicationDescriptor[] = applications.map((app) =>
        cloneApplication(app)
);

export function listAppVncApplications(): AppVncApplicationDescriptor[] {
        return applications.map((app) => cloneApplication(app));
}

export function findAppVncApplication(id: string | null | undefined): AppVncApplicationDescriptor | null {
        if (!id) {
                return null;
        }
        const target = applications.find((app) => app.id === id);
        return target ? cloneApplication(target) : null;
}
