import type {
	RegistryHive,
	RegistryHiveName,
	RegistryKey,
	RegistrySnapshot,
	RegistryValue,
	RegistryValueType
} from '$lib/types/registry';

const baseSnapshot: RegistrySnapshot = {
	HKEY_LOCAL_MACHINE: {
		SOFTWARE: key(
			'HKEY_LOCAL_MACHINE',
			'SOFTWARE',
			null,
			[
				value('InstallationRoot', 'REG_EXPAND_SZ', '%ProgramFiles%\\Tenvy'),
				value('TelemetryOptIn', 'REG_DWORD', '0x00000001', undefined, '2024-04-25T08:10:00Z'),
				value(
					'LastAudit',
					'REG_SZ',
					'2024-05-12T14:22:00Z',
					'ISO timestamp of last review',
					'2024-05-12T14:22:00Z'
				)
			],
			'Core software configuration hive'
		),
		'SOFTWARE\\Policies': key(
			'HKEY_LOCAL_MACHINE',
			'SOFTWARE\\Policies',
			'SOFTWARE',
			[value('(Default)', 'REG_SZ', '', 'Unconfigured policy container')],
			'Administrative policy definitions'
		),
		'SOFTWARE\\Policies\\Microsoft': key(
			'HKEY_LOCAL_MACHINE',
			'SOFTWARE\\Policies\\Microsoft',
			'SOFTWARE\\Policies',
			[value('PolicyState', 'REG_SZ', 'Enforced', undefined, '2024-05-01T09:20:00Z')],
			'Microsoft policy enforcement state'
		),
		'SOFTWARE\\Policies\\Microsoft\\Windows': key(
			'HKEY_LOCAL_MACHINE',
			'SOFTWARE\\Policies\\Microsoft\\Windows',
			'SOFTWARE\\Policies\\Microsoft',
			[
				value(
					'DisableRegistryTools',
					'REG_DWORD',
					'0x00000000',
					'Allow registry editors',
					'2024-04-04T12:00:00Z'
				),
				value('WindowsUpdateMode', 'REG_SZ', 'Managed', undefined, '2024-03-18T17:42:00Z'),
				value(
					'ComplianceTags',
					'REG_MULTI_SZ',
					'Baseline\nHardened\nPrivileged',
					undefined,
					'2024-05-09T11:00:00Z'
				)
			],
			'Windows policy overrides'
		),
		'SOFTWARE\\Microsoft': key(
			'HKEY_LOCAL_MACHINE',
			'SOFTWARE\\Microsoft',
			'SOFTWARE',
			[value('SmartScreenEnabled', 'REG_SZ', 'On', undefined, '2024-04-01T06:15:00Z')],
			'Microsoft platform settings'
		),
		'SOFTWARE\\Microsoft\\Windows NT': key(
			'HKEY_LOCAL_MACHINE',
			'SOFTWARE\\Microsoft\\Windows NT',
			'SOFTWARE\\Microsoft',
			[],
			'Windows NT branch metadata'
		),
		'SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion': key(
			'HKEY_LOCAL_MACHINE',
			'SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion',
			'SOFTWARE\\Microsoft\\Windows NT',
			[
				value('ProductName', 'REG_SZ', 'Windows 11 Pro', undefined, '2024-02-01T10:00:00Z'),
				value('ReleaseId', 'REG_SZ', '24H1', undefined, '2024-02-10T09:15:00Z'),
				value('RegisteredOwner', 'REG_SZ', 'Operations', undefined, '2024-03-01T12:45:00Z'),
				value(
					'InstallDate',
					'REG_QWORD',
					'0x01daff1f8a6b4000',
					'FILETIME of installation',
					'2023-11-10T05:30:00Z'
				)
			],
			'Installed OS details'
		),
		SYSTEM: key(
			'HKEY_LOCAL_MACHINE',
			'SYSTEM',
			null,
			[
				value('CurrentControlSet', 'REG_SZ', 'ControlSet001', undefined, '2024-05-22T15:24:00Z'),
				value(
					'BuildLab',
					'REG_SZ',
					'26100.560.amd64fre.ge_release',
					undefined,
					'2024-05-18T19:30:00Z'
				),
				value('KernelDebug', 'REG_DWORD', '0x00000000', undefined, '2024-01-05T03:14:00Z')
			],
			'System level configuration'
		),
		'SYSTEM\\CurrentControlSet': key(
			'HKEY_LOCAL_MACHINE',
			'SYSTEM\\CurrentControlSet',
			'SYSTEM',
			[],
			'Active control set'
		),
		'SYSTEM\\CurrentControlSet\\Services': key(
			'HKEY_LOCAL_MACHINE',
			'SYSTEM\\CurrentControlSet\\Services',
			'SYSTEM\\CurrentControlSet',
			[],
			'Service control manager state'
		),
		'SYSTEM\\CurrentControlSet\\Services\\TenvyAgent': key(
			'HKEY_LOCAL_MACHINE',
			'SYSTEM\\CurrentControlSet\\Services\\TenvyAgent',
			'SYSTEM\\CurrentControlSet\\Services',
			[
				value('DisplayName', 'REG_SZ', 'Tenvy Agent Service', undefined, '2024-05-20T18:05:00Z'),
				value(
					'ImagePath',
					'REG_EXPAND_SZ',
					'%ProgramFiles%\\Tenvy\\agent.exe',
					undefined,
					'2024-05-20T18:05:00Z'
				),
				value('Start', 'REG_DWORD', '0x00000002', 'Automatic start', '2024-05-20T18:05:00Z'),
				value(
					'Security',
					'REG_BINARY',
					'01 00 04 80 7c 00 00 00 14 00 00 00',
					undefined,
					'2024-05-20T21:00:00Z'
				)
			],
			'Agent service configuration',
			true
		)
	},
	HKEY_CURRENT_USER: {
		SOFTWARE: key(
			'HKEY_CURRENT_USER',
			'SOFTWARE',
			null,
			[
				value('PreferredColorScheme', 'REG_SZ', 'Dark', undefined, '2024-06-02T08:00:00Z'),
				value('LastLogin', 'REG_SZ', '2024-06-02T08:15:00Z', undefined, '2024-06-02T08:15:00Z')
			],
			'User specific software settings',
			false,
			'TEN\Analyst'
		),
		'SOFTWARE\\Tenvy': key(
			'HKEY_CURRENT_USER',
			'SOFTWARE\\Tenvy',
			'SOFTWARE',
			[
				value('PinnedClient', 'REG_SZ', 'alpha-02', undefined, '2024-06-02T08:05:00Z'),
				value(
					'RecentActivity',
					'REG_MULTI_SZ',
					'Bootstrap\nRecon\nCleanup',
					undefined,
					'2024-06-02T07:40:00Z'
				),
				value(
					'SessionTimeout',
					'REG_DWORD',
					'0x0000001e',
					'Timeout in minutes',
					'2024-06-01T12:00:00Z'
				)
			],
			'Controller preferences',
			false,
			'TEN\\Analyst'
		),
		'SOFTWARE\\Tenvy\\RecentSessions': key(
			'HKEY_CURRENT_USER',
			'SOFTWARE\\Tenvy\\RecentSessions',
			'SOFTWARE\\Tenvy',
			[
				value('alpha-02', 'REG_SZ', '2024-06-02T08:10:11Z', undefined, '2024-06-02T08:10:11Z'),
				value('bravo-05', 'REG_SZ', '2024-06-01T19:45:03Z', undefined, '2024-06-01T19:45:03Z'),
				value('charlie-09', 'REG_SZ', '2024-05-28T13:01:54Z', undefined, '2024-05-28T13:01:54Z')
			],
			'Timestamped client activity'
		),
		Environment: key(
			'HKEY_CURRENT_USER',
			'Environment',
			null,
			[
				value(
					'Path',
					'REG_EXPAND_SZ',
					'%USERPROFILE%\\bin;%PATH%',
					undefined,
					'2024-04-30T09:00:00Z'
				),
				value('TENVY_SESSION', 'REG_SZ', 'alpha-02', undefined, '2024-06-02T08:10:00Z'),
				value(
					'TMP',
					'REG_EXPAND_SZ',
					'%USERPROFILE%\\AppData\\Local\\Temp',
					undefined,
					'2024-04-12T15:30:00Z'
				)
			],
			'User environment variables',
			false,
			'TEN\\Analyst'
		)
	},
	HKEY_USERS: {
		'S-1-5-21-1004336348-1177238915-682003330-512': key(
			'HKEY_USERS',
			'S-1-5-21-1004336348-1177238915-682003330-512',
			null,
			[value('Locale', 'REG_SZ', 'en-US', undefined, '2024-02-17T10:00:00Z')],
			'Primary profile'
		),
		'S-1-5-21-1004336348-1177238915-682003330-512\\Software': key(
			'HKEY_USERS',
			'S-1-5-21-1004336348-1177238915-682003330-512\\Software',
			'S-1-5-21-1004336348-1177238915-682003330-512',
			[],
			'Profile software settings'
		),
		'S-1-5-21-1004336348-1177238915-682003330-512\\Software\\TenvyShared': key(
			'HKEY_USERS',
			'S-1-5-21-1004336348-1177238915-682003330-512\\Software\\TenvyShared',
			'S-1-5-21-1004336348-1177238915-682003330-512\\Software',
			[
				value(
					'SharedState',
					'REG_BINARY',
					'10 00 00 00 02 00 00 00',
					undefined,
					'2024-03-11T09:00:00Z'
				),
				value('AllowEscalation', 'REG_DWORD', '0x00000001', undefined, '2024-03-11T09:05:00Z'),
				value('Notes', 'REG_MULTI_SZ', 'Escalate\nAudit\nRotate', undefined, '2024-05-27T14:44:00Z')
			],
			'Cross-profile state sharing'
		)
	}
};

function key(
	hive: RegistryHiveName,
	path: string,
	parentPath: string | null,
	values: RegistryValue[],
	description?: string,
	wow64Mirrored = false,
	owner = 'SYSTEM'
): RegistryKey {
	const timestamps = values
		.map((entry) => Date.parse(entry.lastModified ?? ''))
		.filter((value) => !Number.isNaN(value));
	const lastModified = timestamps.length > 0 ? new Date(Math.max(...timestamps)) : new Date();

	return {
		hive,
		name: path.split('\\').pop() ?? path,
		path,
		parentPath,
		values,
		subKeys: [],
		lastModified: lastModified.toISOString(),
		wow64Mirrored,
		owner,
		description
	} satisfies RegistryKey;
}

function value(
	name: string,
	type: RegistryValueType,
	data: string,
	description?: string,
	modified?: string
): RegistryValue {
	const lastModified = new Date(modified ?? '2024-05-01T09:00:00Z').toISOString();
	return {
		name,
		type,
		data,
		size: estimateSize(type, data),
		lastModified,
		description
	} satisfies RegistryValue;
}

function estimateSize(type: RegistryValueType, data: string): number {
	switch (type) {
		case 'REG_DWORD':
			return 4;
		case 'REG_QWORD':
			return 8;
		case 'REG_BINARY': {
			const sanitized = data.replace(/[^0-9a-fA-F]/g, '');
			return Math.ceil(sanitized.length / 2);
		}
		case 'REG_MULTI_SZ':
			return Math.max(
				2,
				data.split(/\r?\n/).reduce((acc, line) => acc + (line.length + 1) * 2, 2)
			);
		default:
			return data.length * 2;
	}
}

export function createInitialRegistry(): RegistrySnapshot {
	return normalizeSnapshot(structuredClone(baseSnapshot));
}

export function normalizeHive(hive: RegistryHive): RegistryHive {
	const normalized: RegistryHive = {};
	for (const [path, entry] of Object.entries(hive)) {
		normalized[path] = cloneKey(entry);
		normalized[path].subKeys = [];
	}
	for (const entry of Object.values(normalized)) {
		if (entry.parentPath) {
			const parent = normalized[entry.parentPath];
			if (parent) {
				parent.subKeys.push(entry.path);
			}
		}
	}
	for (const entry of Object.values(normalized)) {
		entry.subKeys = entry.subKeys
			.filter(
				(childPath, index, array) => array.indexOf(childPath) === index && normalized[childPath]
			)
			.sort((a, b) => normalized[a].name.localeCompare(normalized[b].name));
	}
	return normalized;
}

function normalizeSnapshot(snapshot: RegistrySnapshot): RegistrySnapshot {
	const result = {} as RegistrySnapshot;
	for (const [hive, hiveData] of Object.entries(snapshot) as [RegistryHiveName, RegistryHive][]) {
		result[hive] = normalizeHive(hiveData);
	}
	return result;
}

function cloneKey(entry: RegistryKey): RegistryKey {
	return {
		...entry,
		values: entry.values.map((item) => ({ ...item })),
		subKeys: [...entry.subKeys]
	} satisfies RegistryKey;
}
