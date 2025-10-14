<script lang="ts">
	import type { DashboardClient } from '$lib/data/dashboard';
	import type { ClientStatus } from '$lib/data/clients';
	import { geoNaturalEarth1, geoPath, geoGraticule10 } from 'd3-geo';
	import { feature } from 'topojson-client';
	import type { Feature, FeatureCollection, GeoJsonProperties, Geometry } from 'geojson';
	import type {
		GeometryCollection as TopologyGeometryCollection,
		Objects,
		Topology
	} from 'topojson-specification';
	import world from 'world-atlas/countries-110m.json';

	type Marker = {
		client: DashboardClient;
		x: number;
		y: number;
		style: { dot: string; halo: string; stroke: string };
	};

	type WorldProperties = GeoJsonProperties & {
		name?: string;
	};

	type WorldObjects = Objects<WorldProperties>;
	type WorldGeometryCollection = TopologyGeometryCollection<WorldProperties>;

	export let clients: DashboardClient[] = [];
	export let highlightCountry: string | null = null;

	const width = 860;
	const height = 460;

	const topology = world as unknown as Topology<WorldObjects>;
	const countriesObject = topology.objects.countries as WorldGeometryCollection;
	const worldFeatures = feature(topology, countriesObject) as FeatureCollection<
		Geometry,
		WorldProperties
	>;

	const projection = geoNaturalEarth1().fitExtent(
		[
			[20, 20],
			[width - 20, height - 20]
		],
		worldFeatures
	);
	const pathGenerator = geoPath(projection);
	const graticulePath = pathGenerator(geoGraticule10()) ?? '';
	const landPaths = worldFeatures.features.map(
		(entry: Feature<Geometry, WorldProperties>, index: number) => ({
			id: String(entry.id ?? entry.properties?.name ?? index),
			d: pathGenerator(entry) ?? ''
		})
	);

	const statusStyles: Record<ClientStatus, Marker['style']> = {
		online: {
			dot: 'rgb(16 185 129)',
			halo: 'rgba(16, 185, 129, 0.18)',
			stroke: 'rgba(16, 185, 129, 0.55)'
		},
		idle: {
			dot: 'rgb(56 189 248)',
			halo: 'rgba(56, 189, 248, 0.22)',
			stroke: 'rgba(56, 189, 248, 0.5)'
		},
		dormant: {
			dot: 'rgb(245 158 11)',
			halo: 'rgba(245, 158, 11, 0.22)',
			stroke: 'rgba(245, 158, 11, 0.55)'
		},
		offline: {
			dot: 'rgb(148 163 184)',
			halo: 'rgba(148, 163, 184, 0.25)',
			stroke: 'rgba(148, 163, 184, 0.45)'
		}
	};

	$: markers = clients
		.map((client) => {
			const coordinates = projection([client.location.longitude, client.location.latitude]);
			if (!coordinates) {
				return null;
			}
			const [x, y] = coordinates;
			return {
				client,
				x,
				y,
				style: statusStyles[client.status]
			} satisfies Marker;
		})
		.filter((entry): entry is Marker => entry !== null);
</script>

<svg
	viewBox={`0 0 ${width} ${height}`}
	role="img"
	aria-label="Client presence map"
	class="h-[320px] w-full overflow-hidden rounded-xl border border-border/60 bg-gradient-to-br from-background via-background to-muted/30"
>
	<defs>
		<radialGradient id="map-glow" cx="50%" cy="50%" r="75%">
			<stop offset="0%" stop-color="rgb(15 23 42 / 0.1)" />
			<stop offset="60%" stop-color="rgb(15 23 42 / 0.04)" />
			<stop offset="100%" stop-color="rgb(15 23 42 / 0.01)" />
		</radialGradient>
	</defs>
	<rect x="0" y="0" {width} {height} fill="url(#map-glow)" />

	{#if graticulePath}
		<path d={graticulePath} class="fill-none stroke-border/40 stroke-[0.4]" />
	{/if}

	{#each landPaths as land (land.id)}
		{#if land.d}
			<path
				d={land.d}
				class="fill-muted/70 stroke-border/50 stroke-[0.5] dark:fill-muted/40 dark:stroke-border/40"
			/>
		{/if}
	{/each}

	{#if markers.length === 0}
		<foreignObject x="0" y="0" {width} {height}>
			<div class="flex h-full items-center justify-center text-sm text-muted-foreground">
				No clients available for this selection.
			</div>
		</foreignObject>
	{/if}

	{#each markers as marker (marker.client.id)}
		<g
			transform={`translate(${marker.x}, ${marker.y})`}
			style={`opacity: ${
				highlightCountry && marker.client.location.countryCode !== highlightCountry ? 0.3 : 1
			};`}
		>
			<circle r="11" style={`fill: ${marker.style.halo};`} />
			<circle
				r="5"
				style={`fill: ${marker.style.dot}; stroke: ${marker.style.stroke}; stroke-width: 1.5;`}
			/>
			<title>
				{marker.client.codename} · {marker.client.location.city}, {marker.client.location.country}
			</title>
		</g>
	{/each}
</svg>
