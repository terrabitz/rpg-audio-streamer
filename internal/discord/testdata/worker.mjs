// Public test protocol fixture; never connects to Discord.
if (process.env.DISCORD_CHANNEL_ID === '2') {
  process.stdout.write('{"state":"error","message":"voice unavailable"}\n')
  process.exitCode = 1
} else if (process.env.DISCORD_CHANNEL_ID === '3') {
  process.stdout.write('invalid status\n')
  setInterval(() => {}, 1000)
} else {
  process.stdout.write('{"state":"connected"}\n')
  process.stdin.resume()
  process.on('SIGTERM', () => process.exit(0))
}
