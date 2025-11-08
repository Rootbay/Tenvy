import { describe, expect, it } from 'vitest';
import nacl from 'tweetnacl';

import type { PluginManifest } from '../../shared/types/plugin-manifest';
import { verifyPluginSignature } from '../../shared/types/plugin-manifest';

const toHex = (value: Uint8Array): string =>
	Array.from(value, (byte) => byte.toString(16).padStart(2, '0')).join('');

const encoder = new TextEncoder();

describe('verifyPluginSignature', () => {
	it('allows sha256 hashes that appear in the allow list', async () => {
		const manifest: PluginManifest = {
			id: 'demo',
			name: 'Demo',
			version: '1.0.0',
			entry: 'demo.dll',
			repositoryUrl: 'https://github.com/rootbay/demo',
			license: { spdxId: 'MIT' },
			requirements: {},
			distribution: {
				defaultMode: 'manual',
				autoUpdate: false,
				signature: 'sha256',
				signatureHash: 'abcdef'
			},
			package: {
				artifact: 'demo.dll',
				hash: 'abcdef'
			}
		};

		const result = await verifyPluginSignature(manifest, {
			sha256AllowList: ['ABCDEF']
		});

		expect(result.trusted).toBe(true);
		expect(result.signatureType).toBe('sha256');
	});

	it('rejects hashes that are not present in the allow list', async () => {
		const manifest: PluginManifest = {
			id: 'demo',
			name: 'Demo',
			version: '1.0.0',
			entry: 'demo.dll',
			repositoryUrl: 'https://github.com/rootbay/demo',
			license: { spdxId: 'MIT' },
			requirements: {},
			distribution: {
				defaultMode: 'manual',
				autoUpdate: false,
				signature: 'sha256',
				signatureHash: 'abcdef'
			},
			package: {
				artifact: 'demo.dll',
				hash: 'abcdef'
			}
		};

                await expect(
                        verifyPluginSignature(manifest, {
                                sha256AllowList: ['deadbeef']
                        })
                ).rejects.toMatchObject({
                        code: 'HASH_NOT_ALLOWED'
                });
	});

	it('verifies ed25519 signatures with known keys', async () => {
		const keyPair = nacl.sign.keyPair.fromSeed(
			Uint8Array.from({ length: 32 }, (_, index) => index + 1)
		);
		const hash = '9e4cba26f4f913a52fcb11f16a34f1db493f9204f0545d01b7a086764d814176';
		const signature = nacl.sign.detached(encoder.encode(hash), keyPair.secretKey);

		const manifest: PluginManifest = {
			id: 'demo',
			name: 'Demo',
			version: '1.0.0',
			entry: 'demo.dll',
			repositoryUrl: 'https://github.com/rootbay/demo',
			license: { spdxId: 'MIT' },
			requirements: {},
			distribution: {
				defaultMode: 'manual',
				autoUpdate: false,
				signature: 'ed25519',
				signatureHash: hash,
				signatureSigner: 'key-1',
				signatureValue: toHex(signature),
				signatureCertificateChain: ['leaf', 'intermediate']
			},
			package: {
				artifact: 'demo.dll',
				hash
			}
		};

		let validated = false;

		const result = await verifyPluginSignature(manifest, {
			ed25519PublicKeys: { 'key-1': keyPair.publicKey },
			certificateValidator: async (chain) => {
				validated = true;
				expect(chain).toEqual(['leaf', 'intermediate']);
			}
		});

		expect(result.trusted).toBe(true);
		expect(result.signatureType).toBe('ed25519');
		expect(result.publicKey).toBe(toHex(keyPair.publicKey));
		expect(validated).toBe(true);
	});

	it('rejects unknown ed25519 signers', async () => {
		const manifest: PluginManifest = {
			id: 'demo',
			name: 'Demo',
			version: '1.0.0',
			entry: 'demo.dll',
			repositoryUrl: 'https://github.com/rootbay/demo',
			license: { spdxId: 'MIT' },
			requirements: {},
			distribution: {
				defaultMode: 'manual',
				autoUpdate: false,
				signature: 'ed25519',
				signatureHash: 'abcdef',
				signatureSigner: 'missing',
				signatureValue: toHex(new Uint8Array(nacl.sign.signatureLength))
			},
			package: {
				artifact: 'demo.dll',
				hash: 'abcdef'
			}
		};

		await expect(verifyPluginSignature(manifest)).rejects.toMatchObject({
			code: 'UNTRUSTED_SIGNER'
		});
	});

	it('rejects ed25519 manifests that omit legacy metadata', async () => {
		const manifest: PluginManifest = {
			id: 'demo',
			name: 'Demo',
			version: '1.0.0',
			entry: 'demo.dll',
			repositoryUrl: 'https://github.com/rootbay/demo',
			license: { spdxId: 'MIT' },
			requirements: {},
			distribution: {
				defaultMode: 'manual',
				autoUpdate: false,
				signature: 'ed25519',
				signatureHash: 'abcdef'
			},
			package: {
				artifact: 'demo.dll',
				hash: 'abcdef'
			}
		};

		await expect(verifyPluginSignature(manifest)).rejects.toMatchObject({
			code: 'UNTRUSTED_SIGNER'
		});
	});
});
