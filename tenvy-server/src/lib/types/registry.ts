export type RegistryHiveName = 'HKEY_LOCAL_MACHINE' | 'HKEY_CURRENT_USER' | 'HKEY_USERS';

export type RegistryValueType =
	| 'REG_SZ'
	| 'REG_EXPAND_SZ'
	| 'REG_MULTI_SZ'
	| 'REG_DWORD'
	| 'REG_QWORD'
	| 'REG_BINARY';

export interface RegistryValue {
	name: string;
	type: RegistryValueType;
	data: string;
	size: number;
	lastModified: string;
	description?: string;
}

export interface RegistryKey {
	hive: RegistryHiveName;
	name: string;
	path: string;
	parentPath: string | null;
	values: RegistryValue[];
	subKeys: string[];
	lastModified: string;
	wow64Mirrored: boolean;
	owner: string;
	description?: string;
}

export type RegistryHive = Record<string, RegistryKey>;

export type RegistrySnapshot = Record<RegistryHiveName, RegistryHive>;
