<script lang="ts">
        import type { Client } from '$lib/data/clients';
        import RemoteDesktopWorkspace from '$lib/components/workspace/tools/remote-desktop/remote-desktop-workspace.svelte';
        import type { RemoteDesktopSessionState } from '$lib/types/remote-desktop';

        let { data } = $props<{
                data: {
                        session: RemoteDesktopSessionState | null;
                        client: Client;
                };
        }>();
</script>

<RemoteDesktopWorkspace client={data.client} initialSession={data.session ?? null} />
