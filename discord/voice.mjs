import { Client, GatewayIntentBits, ChannelType, PermissionFlagsBits } from 'discord.js'
import { entersState, joinVoiceChannel, VoiceConnectionStatus } from '@discordjs/voice'

export async function connectVoice({ token, guildID, channelID, onError }) {
  const client = new Client({ intents: [GatewayIntentBits.Guilds, GatewayIntentBits.GuildVoiceStates] })
  client.on('error', () => onError?.('Discord gateway connection failed'))
  let connection
  try {
    await client.login(token)
    const channel = await client.channels.fetch(channelID)
    if (!channel || channel.type !== ChannelType.GuildVoice || channel.guild.id !== guildID) {
      throw new Error('Configured Discord channel must be a regular voice channel in the configured server')
    }
    const member = await channel.guild.members.fetchMe()
    const permissions = channel.permissionsFor(member)
    if (!permissions?.has([PermissionFlagsBits.ViewChannel, PermissionFlagsBits.Connect, PermissionFlagsBits.Speak])) {
      throw new Error('Bot is missing View Channel, Connect, or Speak permission')
    }
    connection = joinVoiceChannel({
      channelId: channelID,
      guildId: guildID,
      adapterCreator: channel.guild.voiceAdapterCreator,
      selfDeaf: true,
      selfMute: false,
      daveEncryption: true,
    })
    await entersState(connection, VoiceConnectionStatus.Ready, 15_000)
    return {
      client,
      connection,
      close() {
        if (connection.state.status !== VoiceConnectionStatus.Destroyed) connection.destroy()
        client.destroy()
      },
    }
  } catch (error) {
    if (connection && connection.state.status !== VoiceConnectionStatus.Destroyed) connection.destroy()
    client.destroy()
    throw error
  }
}
