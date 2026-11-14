import { createAudioPlayer, NoSubscriberBehavior } from '@discordjs/voice'

export function createMixPlayer() {
  return createAudioPlayer({ behaviors: { noSubscriber: NoSubscriberBehavior.Pause, maxMissedFrames: 100 } })
}
