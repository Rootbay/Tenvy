export interface AgentConfig {
        /**
         * Base interval in milliseconds used by the agent to poll the controller for new work.
         */
        pollIntervalMs: number;
        /**
         * Maximum backoff interval in milliseconds applied when network issues occur.
         */
        maxBackoffMs: number;
        /**
         * Randomisation factor applied to poll intervals to avoid detection patterns.
         */
        jitterRatio: number;
}

export const defaultAgentConfig: AgentConfig = Object.freeze({
        pollIntervalMs: 5_000,
        maxBackoffMs: 60_000,
        jitterRatio: 0.2
});

export interface ServerAgentConfig {
        agent: AgentConfig;
}

export const defaultServerAgentConfig: ServerAgentConfig = Object.freeze({
        agent: defaultAgentConfig
});
