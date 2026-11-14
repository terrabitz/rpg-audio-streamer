export function discordEnvironment(root: string, enabled: boolean, environment?: NodeJS.ProcessEnv): Record<string, string>
export function discordTestDestination(root: string, environment?: NodeJS.ProcessEnv): { guildID: string, channelID: string }
