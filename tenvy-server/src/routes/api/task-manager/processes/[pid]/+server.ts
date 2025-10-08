import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import type { ProcessDetail } from '$lib/types/task-manager';
import { findProcess, toDetail } from '$lib/server/task-manager/process-utils';

export const GET: RequestHandler = async ({ params }) => {
        const raw = params.pid;
        const pid = Number.parseInt(raw ?? '', 10);
        if (!Number.isInteger(pid) || pid <= 0) {
                throw error(400, 'Invalid process identifier');
        }

        const process = await findProcess(pid);
        if (!process) {
                throw error(404, 'Process not found');
        }

        return json(toDetail(process) satisfies ProcessDetail);
};
