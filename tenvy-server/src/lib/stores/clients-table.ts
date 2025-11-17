import { derived, get, writable } from 'svelte/store';
import type { AgentSnapshot } from '../../../../shared/types/agent';
import { registryEventBus, type RegistryEventMessage } from './registry-events';

export type StatusFilter = 'all' | AgentSnapshot['status'];
export type TagFilter = 'all' | string;

export type PageRange = { start: number; end: number };

export type ClientsTableState = {
	agents: AgentSnapshot[];
	searchQuery: string;
	statusFilter: StatusFilter;
	tagFilter: TagFilter;
	perPage: number;
	currentPage: number;
	availableTags: string[];
	filteredAgents: AgentSnapshot[];
	paginatedAgents: AgentSnapshot[];
	pageRange: PageRange;
	totalPages: number;
	paginationItems: (number | 'ellipsis')[];
};

function sanitizeQuery(query: string): string {
	return query.trim().toLowerCase();
}

function matchesStatus(agent: AgentSnapshot, filter: StatusFilter): boolean {
	return filter === 'all' || agent.status === filter;
}

function matchesTag(agent: AgentSnapshot, filter: TagFilter): boolean {
	if (filter === 'all') {
		return true;
	}

	return agent.metadata.tags?.some((tag) => tag.toLowerCase() === filter.toLowerCase()) ?? false;
}

function matchesQuery(agent: AgentSnapshot, query: string): boolean {
	if (!query) {
		return true;
	}

	const haystack = [
		agent.id,
		agent.metadata.hostname,
		agent.metadata.username,
		agent.metadata.os,
		agent.metadata.ipAddress,
		agent.metadata.publicIpAddress,
		...(agent.metadata.tags ?? [])
	]
		.filter(Boolean)
		.map((value) => value!.toString().toLowerCase());

	return haystack.some((value) => value.includes(query));
}

export function filterAgents(
	agents: AgentSnapshot[],
	query: string,
	statusFilter: StatusFilter,
	tagFilter: TagFilter
): AgentSnapshot[] {
	const normalizedQuery = sanitizeQuery(query);
	return agents.filter(
		(agent) =>
			matchesStatus(agent, statusFilter) &&
			matchesTag(agent, tagFilter) &&
			matchesQuery(agent, normalizedQuery)
	);
}

function paginateAgents(
	agents: AgentSnapshot[],
	currentPage: number,
	perPage: number
): { items: AgentSnapshot[]; startIndex: number } {
	const safePerPage = Math.max(1, perPage);
	const safePage = Math.max(1, currentPage);
	const startIndex = (safePage - 1) * safePerPage;
	const slice = agents.slice(startIndex, startIndex + safePerPage);
	return { items: slice, startIndex };
}

function computeAvailableTags(agents: AgentSnapshot[]): string[] {
	const tags = new Set<string>();
	for (const agent of agents) {
		for (const tag of agent.metadata.tags ?? []) {
			tags.add(tag);
		}
	}
	return Array.from(tags).sort((a, b) => a.localeCompare(b));
}

function computePageRange(length: number, sliceLength: number, startIndex: number): PageRange {
	if (length === 0 || sliceLength === 0) {
		return { start: 0, end: 0 };
	}

	const start = startIndex + 1;
	const end = Math.min(startIndex + sliceLength, length);
	return { start, end };
}

export function buildPaginationItems(
	total: number,
	current: number,
	siblingCount = 1
): (number | 'ellipsis')[] {
	if (total <= 1) {
		return [1];
	}

	const safeCurrent = Math.min(Math.max(current, 1), total);
	const start = Math.max(2, safeCurrent - siblingCount);
	const end = Math.min(total - 1, safeCurrent + siblingCount);

	const items: (number | 'ellipsis')[] = [1];

	if (start > 2) {
		items.push('ellipsis');
	}

	for (let page = start; page <= end; page += 1) {
		items.push(page);
	}

	if (end < total - 1) {
		items.push('ellipsis');
	}

	items.push(total);

	return items;
}

// Remove duplicates while keeping the latest snapshot for each agent id.
function dedupeAgents(agents: AgentSnapshot[]): AgentSnapshot[] {
	const seen = new Set<string>();
	const result: AgentSnapshot[] = [];
	for (let index = agents.length - 1; index >= 0; index -= 1) {
		const agent = agents[index];
		if (seen.has(agent.id)) {
			continue;
		}
		seen.add(agent.id);
		result.unshift(agent);
	}
	return result;
}

function upsertAgent(list: AgentSnapshot[], next: AgentSnapshot): AgentSnapshot[] {
	const clone = [...list];
	const index = clone.findIndex((agent) => agent.id === next.id);
	if (index === -1) {
		clone.push(next);
	} else {
		clone[index] = next;
	}
	return clone;
}

export type ClientsTableStore = ReturnType<typeof createClientsTableStore>;

export function createClientsTableStore(initialAgents: AgentSnapshot[]): {
	subscribe: (
		run: (value: ClientsTableState) => void,
		invalidate?: (value?: ClientsTableState) => void
	) => () => void;
	setAgents: (agents: AgentSnapshot[]) => void;
	setSearchQuery: (value: string) => void;
	setStatusFilter: (value: StatusFilter) => void;
	setTagFilter: (value: TagFilter) => void;
	setPerPage: (value: number) => void;
	goToPage: (page: number) => void;
	nextPage: () => void;
	previousPage: () => void;
	optimisticUpdateAgent: (agent: AgentSnapshot) => void;
	isOptimistic: (agentId: string) => boolean;
	clearOptimisticAgent: (agentId: string) => void;
} {
	const agents = writable(dedupeAgents(initialAgents ?? []));
	const searchQuery = writable('');
	const statusFilter = writable<StatusFilter>('all');
	const tagFilter = writable<TagFilter>('all');
	const perPage = writable(10);
	const currentPage = writable(1);

	derived([searchQuery, statusFilter, tagFilter, perPage], () => {
		currentPage.set(1);
	}).subscribe(() => {
		// no-op subscription to keep the derived store active
	});

	const state = derived(
		[agents, searchQuery, statusFilter, tagFilter, perPage, currentPage],
		([$agents, $searchQuery, $statusFilter, $tagFilter, $perPage, $currentPage]) => {
			const availableTags = computeAvailableTags($agents);
			const filteredAgents = filterAgents($agents, $searchQuery, $statusFilter, $tagFilter);

			const totalPages =
				filteredAgents.length === 0
					? 1
					: Math.max(1, Math.ceil(filteredAgents.length / Math.max(1, $perPage)));

			const safeCurrentPage = Math.min(Math.max($currentPage, 1), totalPages);
			if (safeCurrentPage !== $currentPage) {
				currentPage.set(safeCurrentPage);
			}

			const { items: paginatedAgents, startIndex } = paginateAgents(
				filteredAgents,
				safeCurrentPage,
				Math.max(1, $perPage)
			);

			const pageRange = computePageRange(filteredAgents.length, paginatedAgents.length, startIndex);
			const paginationItems = buildPaginationItems(totalPages, safeCurrentPage);

			return {
				agents: $agents,
				searchQuery: $searchQuery,
				statusFilter: $statusFilter,
				tagFilter: $tagFilter,
				perPage: Math.max(1, $perPage),
				currentPage: safeCurrentPage,
				availableTags,
				filteredAgents,
				paginatedAgents,
				pageRange,
				totalPages,
				paginationItems
			} satisfies ClientsTableState;
		}
	);

	let subscribers = 0;
	let busUnsubscribe: (() => void) | null = null;
	const optimisticAgents = new Map<string, AgentSnapshot>();

	const applyRegistryEvent = (event: RegistryEventMessage) => {
		if (!event || typeof event !== 'object') {
			return;
		}

		if (event.type === 'agents') {
			optimisticAgents.clear();
			agents.set(dedupeAgents(event.agents ?? []));
			return;
		}

		if (event.type === 'agent') {
			if (event.optimistic) {
				optimisticAgents.set(event.agent.id, event.agent);
			} else {
				optimisticAgents.delete(event.agent.id);
			}
			agents.update((current) => dedupeAgents(upsertAgent(current, event.agent)));
		}
	};

	const startBus = () => {
		if (busUnsubscribe) {
			return;
		}
		busUnsubscribe = registryEventBus.subscribe((event) => {
			applyRegistryEvent(event);
		});
	};

	const stopBus = () => {
		busUnsubscribe?.();
		busUnsubscribe = null;
		optimisticAgents.clear();
	};

	return {
		subscribe: (run, invalidate) => {
			const unsubscribe = state.subscribe(run, invalidate);
			subscribers += 1;
			if (subscribers === 1) {
				startBus();
			}
			return () => {
				unsubscribe();
				subscribers = Math.max(0, subscribers - 1);
				if (subscribers === 0) {
					stopBus();
				}
			};
		},
		setAgents: (nextAgents) => {
			optimisticAgents.clear();
			agents.set(dedupeAgents(nextAgents ?? []));
		},
		setSearchQuery: (value) => searchQuery.set(value),
		setStatusFilter: (value) => statusFilter.set(value),
		setTagFilter: (value) => tagFilter.set(value),
		setPerPage: (value) => perPage.set(Math.max(1, value)),
		goToPage: (page) => currentPage.set(Math.max(1, Math.trunc(page))),
		nextPage: () => {
			const { currentPage: page, totalPages } = get(state);
			currentPage.set(Math.min(totalPages, page + 1));
		},
		previousPage: () => {
			const { currentPage: page } = get(state);
			currentPage.set(Math.max(1, page - 1));
		},
		optimisticUpdateAgent: (agent: AgentSnapshot) => {
			registryEventBus.emitOptimistic({
				type: 'agent',
				agent
			} as RegistryEventMessage);
		},
		isOptimistic: (agentId: string) => optimisticAgents.has(agentId),
		clearOptimisticAgent: (agentId: string) => {
			optimisticAgents.delete(agentId);
		}
	};
}
